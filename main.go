package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"playbook/internal/runner"
	"playbook/internal/server"
	"playbook/internal/store"
)

//go:embed content
var contentFS embed.FS

//go:embed web
var webFS embed.FS

func main() {
	if len(os.Args) > 1 && os.Args[1] == "toolchains" {
		os.Exit(toolchainsCmd(os.Args[2:]))
	}

	port := flag.Int("port", 0, "port to listen on (default: course.json \"port\", else 4358)")
	host := flag.String("host", "127.0.0.1", "address to bind (0.0.0.0 for Docker/LAN — the playground runs arbitrary code, only expose on trusted networks)")
	dataDir := flag.String("data", "", "directory for user data (default: OS config dir)")
	envDir := flag.String("env-dir", "", "the playbook's private toolchain environment, $ENV_DIR (default: $PLAYBOOK_ENV_DIR, else <data>/env)")
	noOpen := flag.Bool("no-open", false, "do not open the browser automatically")
	flag.Parse()

	appID, coursePort := courseMeta()
	if *port == 0 {
		*port = coursePort
	}
	if *port == 0 {
		*port = 4358
	}

	*dataDir = resolveDataDir(*dataDir, appID)
	db, err := store.Open(filepath.Join(*dataDir, "user.db"))
	if err != nil {
		log.Fatalf("cannot open database: %v", err)
	}
	defer db.Close()

	toolchains := loadToolchains(*dataDir, *envDir)

	content, err := fs.Sub(contentFS, "content")
	if err != nil {
		log.Fatal(err)
	}
	web, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}

	srv := server.New(content, web, db, toolchains)
	addr := fmt.Sprintf("%s:%d", *host, *port)
	url := fmt.Sprintf("http://127.0.0.1:%d", *port)

	if !*noOpen {
		go func() {
			time.Sleep(300 * time.Millisecond)
			openBrowser(url)
		}()
	}

	log.Printf("%s running at %s  (data: %s, env: %s)", appID, url, *dataDir, toolchains.EnvDir())
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatal(err)
	}
}

// courseMeta reads app_id and port from the embedded course.json.
func courseMeta() (appID string, port int) {
	appID = "playbook"
	if b, err := contentFS.ReadFile("content/course.json"); err == nil {
		var m struct {
			AppID string `json:"app_id"`
			Port  int    `json:"port"`
		}
		if json.Unmarshal(b, &m) == nil {
			if m.AppID != "" {
				appID = m.AppID
			}
			port = m.Port
		}
	}
	return appID, port
}

func resolveDataDir(dir, appID string) string {
	if dir == "" {
		cfg, err := os.UserConfigDir()
		if err != nil {
			log.Fatalf("cannot determine config dir: %v (use --data)", err)
		}
		dir = filepath.Join(cfg, appID)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatalf("cannot create data dir %s: %v", dir, err)
	}
	return dir
}

// loadToolchains reads content/toolchains.json. $ENV_DIR precedence:
// --env-dir, then $PLAYBOOK_ENV_DIR (Docker bakes one into the image), then
// <data>/env. Persistent workspaces live in <data>/workspaces.
func loadToolchains(dataDir, envDir string) *runner.Registry {
	if envDir == "" {
		envDir = os.Getenv("PLAYBOOK_ENV_DIR")
	}
	if envDir == "" {
		envDir = filepath.Join(dataDir, "env")
	}
	// commands run inside the workspace, so relative paths would break
	envDir, err := filepath.Abs(envDir)
	if err != nil {
		log.Fatal(err)
	}
	if dataDir, err = filepath.Abs(dataDir); err != nil {
		log.Fatal(err)
	}
	b, err := contentFS.ReadFile("content/toolchains.json")
	if err != nil {
		log.Fatalf("cannot read content/toolchains.json: %v", err)
	}
	reg, err := runner.Load(b, envDir, filepath.Join(dataDir, "workspaces"))
	if err != nil {
		log.Fatal(err)
	}
	return reg
}

// toolchainsCmd implements `playbook toolchains [list|verify [name...]]`:
// the playbook-setup-env skill's way to check the environment with the
// exact definitions the playground uses.
func toolchainsCmd(args []string) int {
	fset := flag.NewFlagSet("toolchains", flag.ExitOnError)
	dataDir := fset.String("data", "", "directory for user data (default: OS config dir)")
	envDir := fset.String("env-dir", "", "override $ENV_DIR (default: $PLAYBOOK_ENV_DIR, else <data>/env)")
	fset.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: playbook toolchains [--data DIR] [--env-dir DIR] [list | verify [name...]]")
		fset.PrintDefaults()
	}
	// flags may come anywhere: pull them in front of the positionals
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case !strings.HasPrefix(a, "-"):
			pos = append(pos, a)
		case strings.Contains(a, "=") || i+1 >= len(args):
			flags = append(flags, a)
		default: // every flag here takes a value
			flags = append(flags, a, args[i+1])
			i++
		}
	}
	_ = fset.Parse(flags)
	sub := ""
	if len(pos) > 0 {
		sub = pos[0]
	}
	appID, _ := courseMeta()
	reg := loadToolchains(resolveDataDir(*dataDir, appID), *envDir)
	avail := reg.Available()

	switch sub {
	case "", "list":
		fmt.Printf("ENV_DIR=%s\n", reg.EnvDir())
		for _, tc := range reg.Toolchains() {
			state := "installed"
			if !avail[tc.Name] {
				state = "missing (" + strings.Join(tc.Detect, ", ") + ")"
			}
			fmt.Printf("%-14s %-24s %s\n", tc.Name, tc.Label, state)
		}
		return 0
	case "verify":
		names := pos[1:]
		explicit := len(names) > 0
		if !explicit {
			for _, tc := range reg.Toolchains() {
				names = append(names, tc.Name)
			}
		}
		failed := false
		for _, name := range names {
			if ok, known := avail[name]; known && !ok && !explicit {
				fmt.Printf("SKIP %s (not installed)\n", name)
				continue
			}
			out, err := verifyToolchain(reg, name)
			if err != nil {
				failed = true
				fmt.Printf("FAIL %s: %v\n", name, err)
				if out != "" {
					fmt.Println(indent(out))
				}
				continue
			}
			// exit 0 alone isn't proof: show what the scratch printed
			fmt.Printf("PASS %s\n%s\n", name, indent(out))
		}
		if failed {
			return 1
		}
		return 0
	default:
		fset.Usage()
		return 2
	}
}

// verifyToolchain runs the toolchain's scratch program through the real
// runner and expects exit code 0.
func verifyToolchain(reg *runner.Registry, name string) (string, error) {
	var tc *runner.Toolchain
	for _, t := range reg.Toolchains() {
		if t.Name == name {
			tc = t
		}
	}
	if tc == nil {
		return "", fmt.Errorf("not defined in content/toolchains.json")
	}
	if len(tc.Scratch) == 0 {
		return "", fmt.Errorf("no scratch program to verify with")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	sess := reg.NewSession(ctx)
	defer sess.Close()

	var mu sync.Mutex
	var out strings.Builder
	collect := func(s string) { mu.Lock(); out.WriteString(s); mu.Unlock() }
	exit := make(chan int, 1)
	if err := sess.Run(name, tc.Scratch, nil, collect, func(string) {}, func(c int) { exit <- c }); err != nil {
		return out.String(), err
	}
	select {
	case code := <-exit:
		mu.Lock()
		defer mu.Unlock()
		if code != 0 {
			return out.String(), fmt.Errorf("exit %d", code)
		}
		return out.String(), nil
	case <-ctx.Done():
		sess.Kill()
		return out.String(), fmt.Errorf("timed out")
	}
}

func indent(s string) string {
	return "    " + strings.ReplaceAll(strings.TrimRight(s, "\n"), "\n", "\n    ")
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
