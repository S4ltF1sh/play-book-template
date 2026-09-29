// Package runner compiles and runs playground programs locally, streaming
// their output through a pty so stdio is unbuffered and interactive.
// Multiple toolchains are supported; each pane declares one (or it is
// inferred from the entry file's extension).
package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

const runTimeout = 5 * time.Minute

type File struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// Toolchain describes how to build and run one language. The first file in
// the pane's list is the entry point.
type Toolchain struct {
	Name string
	// Exts are the file extensions written into the workdir (others are
	// silently dropped so stray names can't land on disk).
	Exts []string
	// Bin is the executable whose presence means the toolchain is installed.
	Bin string
	// Compile returns the compile command, or nil for interpreted languages.
	Compile func(entry string, srcs []string) *exec.Cmd
	// Run returns the command that executes the program.
	Run func(entry string, args []string) *exec.Cmd
}

func command(name string, args ...string) *exec.Cmd { return exec.Command(name, args...) }

// kotlinMainClass derives the class kotlinc produces for an entry file:
// main.kt -> MainKt.
func kotlinMainClass(entry string) string {
	base := strings.TrimSuffix(filepath.Base(entry), filepath.Ext(entry))
	return strings.ToUpper(base[:1]) + base[1:] + "Kt"
}

var toolchains = map[string]*Toolchain{
	"c": {
		Name: "c", Exts: []string{".c", ".h"}, Bin: "cc",
		Compile: func(entry string, srcs []string) *exec.Cmd {
			return command("cc", append([]string{"-Wall", "-Wextra", "-O0", "-o", "prog"}, srcs...)...)
		},
		Run: func(entry string, args []string) *exec.Cmd {
			return command("./prog", args...)
		},
	},
	"cpp": {
		Name: "cpp", Exts: []string{".cpp", ".cc", ".cxx", ".hpp", ".h"}, Bin: "c++",
		Compile: func(entry string, srcs []string) *exec.Cmd {
			return command("c++", append([]string{"-std=c++17", "-Wall", "-Wextra", "-O0", "-o", "prog"}, srcs...)...)
		},
		Run: func(entry string, args []string) *exec.Cmd {
			return command("./prog", args...)
		},
	},
	"python": {
		Name: "python", Exts: []string{".py", ".txt", ".json", ".csv"}, Bin: "python3",
		Run: func(entry string, args []string) *exec.Cmd {
			return command("python3", append([]string{"-u", entry}, args...)...)
		},
	},
	"node": {
		Name: "node", Exts: []string{".js", ".mjs", ".cjs", ".json", ".txt"}, Bin: "node",
		Run: func(entry string, args []string) *exec.Cmd {
			return command("node", append([]string{entry}, args...)...)
		},
	},
	"kotlin": {
		Name: "kotlin", Exts: []string{".kt", ".kts"}, Bin: "kotlinc",
		Compile: func(entry string, srcs []string) *exec.Cmd {
			if strings.HasSuffix(entry, ".kts") {
				return nil // scripts run directly via kotlinc -script
			}
			return command("kotlinc", append(append([]string{}, srcs...), "-d", "prog.jar")...)
		},
		Run: func(entry string, args []string) *exec.Cmd {
			if strings.HasSuffix(entry, ".kts") {
				return command("kotlinc", append([]string{"-script", entry}, args...)...)
			}
			// kotlin CLI launcher; entry main.kt -> class MainKt
			return command("kotlin", append([]string{"-classpath", "prog.jar", kotlinMainClass(entry)}, args...)...)
		},
	},
	"java": {
		Name: "java", Exts: []string{".java"}, Bin: "javac",
		Compile: func(entry string, srcs []string) *exec.Cmd {
			return command("javac", append([]string{"-d", "."}, srcs...)...)
		},
		Run: func(entry string, args []string) *exec.Cmd {
			cls := strings.TrimSuffix(filepath.Base(entry), ".java")
			return command("java", append([]string{cls}, args...)...)
		},
	},
}

// extToToolchain maps SOURCE extensions (compiled/executed files) to their
// toolchain. Headers (.h/.hpp) are deliberately absent: they are staged into
// the workdir via Exts but never passed to the compiler or picked as entry.
var extToToolchain = map[string]string{
	".c":    "c",
	".cpp":  "cpp",
	".cc":   "cpp",
	".cxx":  "cpp",
	".py":   "python",
	".js":   "node",
	".mjs":  "node",
	".cjs":  "node",
	".kt":   "kotlin",
	".kts":  "kotlin",
	".java": "java",
}

// Resolve picks the toolchain: explicit name first, else inferred from the
// first file's extension, else "c" for backward compatibility.
func Resolve(name string, files []File) (*Toolchain, error) {
	if name == "" && len(files) > 0 {
		name = extToToolchain[filepath.Ext(files[0].Name)]
	}
	if name == "" {
		name = "c"
	}
	tc, ok := toolchains[name]
	if !ok {
		return nil, fmt.Errorf("unknown toolchain %q", name)
	}
	if _, err := exec.LookPath(tc.Bin); err != nil {
		return nil, fmt.Errorf("toolchain %q is not installed (missing %q) — run the playbook-setup-env skill / toolchain %q chưa được cài trên máy", tc.Name, tc.Bin, tc.Name)
	}
	return tc, nil
}

// Available reports which known toolchains are installed on this machine.
func Available() map[string]bool {
	out := map[string]bool{}
	for name, tc := range toolchains {
		_, err := exec.LookPath(tc.Bin)
		out[name] = err == nil
	}
	return out
}

// Session is one playground pane: at most one running program at a time.
type Session struct {
	mu   sync.Mutex
	cmd  *exec.Cmd
	ptmx *os.File
	dir  string
}

func New() *Session { return &Session{} }

// Run stages files, compiles if the toolchain needs it, and starts the
// program. Events are delivered via the callbacks; onExit fires exactly once
// per successful start.
func (s *Session) Run(toolchain string, files []File, args []string, onOut func(string), onStatus func(string), onExit func(int)) error {
	s.Kill()

	s.mu.Lock()
	defer s.mu.Unlock()

	tc, err := Resolve(toolchain, files)
	if err != nil {
		return err
	}

	dir, err := os.MkdirTemp("", "playbook-run-*")
	if err != nil {
		return err
	}
	s.dir = dir

	allowed := func(name string) bool {
		for _, e := range tc.Exts {
			if strings.HasSuffix(name, e) {
				return true
			}
		}
		return false
	}
	var srcs []string
	entry := ""
	for _, f := range files {
		name := filepath.Base(f.Name) // no path traversal
		if !allowed(name) {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(f.Content), 0o644); err != nil {
			return err
		}
		if extToToolchain[filepath.Ext(name)] == tc.Name {
			if entry == "" {
				entry = name // first source file listed = entry point
			}
			srcs = append(srcs, name)
		}
	}
	if entry == "" {
		return fmt.Errorf("no %s source file to run / không có file nguồn %s nào để chạy", tc.Name, tc.Name)
	}

	if tc.Compile != nil {
		if cc := tc.Compile(entry, srcs); cc != nil {
			onStatus("compiling")
			cc.Dir = dir
			out, err := cc.CombinedOutput()
			if len(out) > 0 {
				onOut(string(out))
			}
			if err != nil {
				return fmt.Errorf("compile failed / biên dịch thất bại")
			}
		}
	}

	cmd := tc.Run(entry, args)
	cmd.Dir = dir
	ptmx, err := pty.Start(cmd) // Setsid+Setctty: prog leads its own process group
	if err != nil {
		return err
	}
	s.cmd = cmd
	s.ptmx = ptmx
	onStatus("running")

	timer := time.AfterFunc(runTimeout, func() { s.Kill() })

	go func() {
		buf := make([]byte, 4096)
		for {
			n, rerr := ptmx.Read(buf)
			if n > 0 {
				onOut(string(buf[:n]))
			}
			if rerr != nil { // EOF or pty closed
				break
			}
		}
		err := cmd.Wait()
		timer.Stop()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			code = -1
		}
		s.mu.Lock()
		if s.cmd == cmd {
			s.cmd, s.ptmx = nil, nil
		}
		s.mu.Unlock()
		os.RemoveAll(dir)
		onExit(code)
	}()
	return nil
}

// Stdin writes user input to the running program.
func (s *Session) Stdin(data string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ptmx != nil {
		_, _ = s.ptmx.WriteString(data)
	}
}

// Kill terminates the running program (and its forked children) if any.
func (s *Session) Kill() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil && s.cmd.Process != nil {
		// negative pid = whole process group (covers fork()ed children)
		_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGKILL)
	}
	if s.ptmx != nil {
		_ = s.ptmx.Close()
	}
}
