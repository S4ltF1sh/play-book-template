package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// presets loads the template's own content/toolchains.json.
func presets(t *testing.T) *Registry {
	t.Helper()
	b, err := os.ReadFile("../../content/toolchains.json")
	if err != nil {
		t.Fatal(err)
	}
	r, err := Load(b, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("Load presets: %v", err)
	}
	return r
}

// shReg defines a language-free toolchain on plain sh, so the engine is
// tested on any POSIX machine.
func shReg(t *testing.T) *Registry {
	t.Helper()
	r, err := Load([]byte(`[
	  { "name": "sh", "detect": ["sh"], "sources": ["*.sh"], "stage": ["*.txt"],
	    "layout": { "*.txt": "data" },
	    "env": { "GREETING": "hi from $ENV_DIR" },
	    "build": "echo building $SRCS",
	    "run": "exec sh \"$ENTRY\" \"$@\"" },
	  { "name": "sh-persist", "detect": ["sh"], "sources": ["*.sh"], "workspace": "persistent",
	    "run": "exec sh \"$ENTRY\" \"$@\"" },
	  { "name": "sh-broken", "detect": ["sh"], "sources": ["*.sh"],
	    "build": "echo cannot build; exit 3", "run": "true" },
	  { "name": "ghost", "detect": ["definitely-not-installed-xyz"], "sources": ["*.ghost"], "run": "true" }
	]`), "/tmp/env-dir-for-test", t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return r
}

// runToExit runs files under toolchain and returns (output, exitCode).
// Skips the test when the toolchain isn't installed on this machine.
func runToExit(t *testing.T, reg *Registry, sess *Session, toolchain string, files []File, args []string) (string, int) {
	t.Helper()
	if _, err := reg.Resolve(toolchain, files); err != nil {
		t.Skipf("skipping: %v", err)
	}
	if sess == nil {
		sess = reg.NewSession(context.Background())
		defer sess.Close()
	}
	var mu sync.Mutex
	var out strings.Builder
	exit := make(chan int, 1)
	err := sess.Run(toolchain, files, args,
		func(o string) { mu.Lock(); out.WriteString(o); mu.Unlock() },
		func(string) {},
		func(code int) { exit <- code },
	)
	if err != nil {
		t.Fatalf("Run: %v\noutput:\n%s", err, out.String())
	}
	select {
	case code := <-exit:
		mu.Lock()
		defer mu.Unlock()
		return out.String(), code
	case <-time.After(90 * time.Second):
		sess.Kill()
		t.Fatal("program did not exit in time")
		return "", -1
	}
}

func wantOutput(t *testing.T, reg *Registry, toolchain string, files []File, args []string, want string) {
	t.Helper()
	out, code := runToExit(t, reg, nil, toolchain, files, args)
	if code != 0 {
		t.Fatalf("exit %d, output:\n%s", code, out)
	}
	if !strings.Contains(out, want) {
		t.Fatalf("output missing %q:\n%s", want, out)
	}
}

// --- engine (language-free) ---

// Build output streams before the program's; $SRCS, layout, env
// expansion and args with spaces all reach the commands intact.
func TestEngineCommandsAndLayout(t *testing.T) {
	reg := shReg(t)
	out, code := runToExit(t, reg, nil, "sh", []File{
		{Name: "main.sh", Content: `echo "entry=$ENTRY"; echo "arg1=$1"; echo "$GREETING"; cat data/msg.txt; ls helper.sh` + "\n"},
		{Name: "helper.sh", Content: "true\n"},
		{Name: "msg.txt", Content: "staged into data/\n"},
		{Name: "ignored.bin", Content: "dropped"},
	}, []string{"two words", "x"})
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	for _, want := range []string{
		"building main.sh helper.sh",
		"entry=main.sh",
		"arg1=two words",
		"hi from /tmp/env-dir-for-test",
		"staged into data/",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "building") > strings.Index(out, "entry=") {
		t.Errorf("build output should come first:\n%s", out)
	}
}

func TestEngineBuildFailure(t *testing.T) {
	reg := shReg(t)
	sess := reg.NewSession(context.Background())
	defer sess.Close()
	var out strings.Builder
	err := sess.Run("sh-broken", []File{{Name: "main.sh"}}, nil,
		func(o string) { out.WriteString(o) }, func(string) {}, func(int) { t.Error("onExit after failed build") })
	if err == nil || !strings.Contains(out.String(), "cannot build") {
		t.Fatalf("want compile error with build output, got err=%v out=%q", err, out.String())
	}
}

// A persistent workspace survives between Runs of one session; a second
// session with the same files gets its own temp dir instead.
func TestEnginePersistentWorkspace(t *testing.T) {
	reg := shReg(t)
	files := []File{{Name: "main.sh", Content: "[ -f marker ] && echo reused; touch marker; echo ok\n"}}
	sess := reg.NewSession(context.Background())
	defer sess.Close()
	if out, _ := runToExit(t, reg, sess, "sh-persist", files, nil); strings.Contains(out, "reused") {
		t.Fatalf("first run saw a marker:\n%s", out)
	}
	if out, _ := runToExit(t, reg, sess, "sh-persist", files, nil); !strings.Contains(out, "reused") {
		t.Fatalf("second run did not reuse the workspace:\n%s", out)
	}
	other := reg.NewSession(context.Background())
	defer other.Close()
	if out, _ := runToExit(t, reg, other, "sh-persist", files, nil); strings.Contains(out, "reused") {
		t.Fatalf("a concurrent session shared the claimed workspace:\n%s", out)
	}
}

func TestEngineRejectsPathTraversal(t *testing.T) {
	reg := shReg(t)
	sess := reg.NewSession(context.Background())
	defer sess.Close()
	for _, name := range []string{"../evil.sh", "/tmp/evil.sh"} {
		err := sess.Run("sh", []File{{Name: name, Content: "true"}}, nil, func(string) {}, func(string) {}, func(int) {})
		if err == nil || !strings.Contains(err.Error(), "invalid file name") {
			t.Errorf("%s: want invalid file name error, got %v", name, err)
		}
	}
}

func TestEngineResolve(t *testing.T) {
	reg := shReg(t)
	if tc, err := reg.Resolve("", []File{{Name: "x.sh"}}); err != nil || tc.Name != "sh" {
		t.Fatalf("inference: got %v, %v; want first matching definition \"sh\"", tc, err)
	}
	if _, err := reg.Resolve("ghost", nil); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("want not-installed error, got %v", err)
	}
	if _, err := reg.Resolve("nope", nil); err == nil || !strings.Contains(err.Error(), "unknown toolchain") {
		t.Fatalf("want unknown-toolchain error, got %v", err)
	}
	if avail := reg.Available(); !avail["sh"] || avail["ghost"] {
		t.Fatalf("Available = %v", avail)
	}
}

// $ENV_DIR/bin is searched before PATH, for detection and for commands.
func TestEngineEnvDirBin(t *testing.T) {
	env := t.TempDir()
	if err := os.MkdirAll(filepath.Join(env, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env, "bin", "mytool"), []byte("#!/bin/sh\necho mytool ran\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	reg, err := Load([]byte(`[{ "name": "t", "detect": ["mytool"], "sources": ["*.x"], "run": "exec mytool" }]`), env, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	wantOutput(t, reg, "t", []File{{Name: "a.x"}}, nil, "mytool ran")
}

func TestLoadValidation(t *testing.T) {
	for name, src := range map[string]string{
		"empty":         `[]`,
		"no name":       `[{"sources":["*.c"],"run":"x"}]`,
		"duplicate":     `[{"name":"a","sources":["*.c"],"run":"x"},{"name":"a","sources":["*.c"],"run":"x"}]`,
		"no run":        `[{"name":"a","sources":["*.c"]}]`,
		"no sources":    `[{"name":"a","run":"x"}]`,
		"bad workspace": `[{"name":"a","sources":["*.c"],"run":"x","workspace":"forever"}]`,
		"escaping dir":  `[{"name":"a","sources":["*.c"],"run":"x","layout":{"*.c":"../out"}}]`,
	} {
		if _, err := Load([]byte(src), "", ""); err == nil {
			t.Errorf("%s: expected a validation error", name)
		}
	}
}

// --- presets shipped in content/toolchains.json (skip when not installed) ---

// Every preset's scratch program must run — the same smoke test as
// `playbook toolchains verify`.
func TestPresetScratch(t *testing.T) {
	reg := presets(t)
	for _, tc := range reg.Toolchains() {
		t.Run(tc.Name, func(t *testing.T) {
			if len(tc.Scratch) == 0 {
				t.Fatal("preset has no scratch program")
			}
			wantOutput(t, reg, tc.Name, tc.Scratch, nil, "hello, playground!")
		})
	}
}

// C: a header must be staged for #include but never passed to the compiler.
func TestCWithHeader(t *testing.T) {
	wantOutput(t, presets(t), "c", []File{
		{Name: "main.c", Content: "#include <stdio.h>\n#include \"greet.h\"\nint main(void){printf(\"%s\\n\", GREETING);return 0;}\n"},
		{Name: "greet.h", Content: "#define GREETING \"hi from c\"\n"},
	}, nil, "hi from c")
}

func TestPythonArgs(t *testing.T) {
	wantOutput(t, presets(t), "python", []File{
		{Name: "main.py", Content: "import sys\nprint('hi from', sys.argv[1])\n"},
	}, []string{"python"}, "hi from python")
}

// Rust: only the crate root is compiled; sibling modules resolve via `mod`.
func TestRustWithModule(t *testing.T) {
	wantOutput(t, presets(t), "rust", []File{
		{Name: "main.rs", Content: "mod greet;\nfn main(){println!(\"{}\", greet::GREETING);}\n"},
		{Name: "greet.rs", Content: "pub const GREETING: &str = \"hi from rust\";\n"},
	}, nil, "hi from rust")
}

// Extension inference follows toolchains.json order: .rs -> "rust", not "rust-cargo".
func TestPresetInference(t *testing.T) {
	reg := presets(t)
	for file, want := range map[string]string{"x.cpp": "cpp", "x.rs": "rust", "x.kts": "kotlin", "x.h": ""} {
		got := ""
		for _, tc := range reg.Toolchains() {
			if matchAny(tc.Sources, file) {
				got = tc.Name
				break
			}
		}
		if got != want {
			t.Errorf("%s inferred %q, want %q", file, got, want)
		}
	}
}

// Cargo: .rs files land in src/ (layout), sibling modules resolve, args
// pass through `cargo run -- "$@"`, and the persistent workspace keeps
// target/ so the second Run skips the full build.
func TestRustCargoPersistent(t *testing.T) {
	reg := presets(t)
	files := []File{
		{Name: "main.rs", Content: "mod greet;\nfn main(){let a: Vec<String> = std::env::args().skip(1).collect(); println!(\"{} {}\", greet::GREETING, a.join(\"|\"));}\n"},
		{Name: "greet.rs", Content: "pub const GREETING: &str = \"hi from cargo\";\n"},
		{Name: "Cargo.toml", Content: "[package]\nname = \"app\"\nversion = \"0.1.0\"\nedition = \"2021\"\n\n[dependencies]\n"},
	}
	sess := reg.NewSession(context.Background())
	defer sess.Close()
	var took [2]time.Duration
	for i := range took {
		start := time.Now()
		out, code := runToExit(t, reg, sess, "rust-cargo", files, []string{"a b", "c"})
		took[i] = time.Since(start)
		if code != 0 || !strings.Contains(out, "hi from cargo a b|c") {
			t.Fatalf("run %d: exit %d, output:\n%s", i+1, code, out)
		}
	}
	t.Logf("first run %v, second run %v", took[0], took[1])
	if took[1] >= took[0] {
		t.Errorf("second run (%v) not faster than first (%v): workspace not reused?", took[1], took[0])
	}
}

// Kotlin: all .kt files compile together; entry main.kt runs as MainKt.
func TestKotlinMultiFile(t *testing.T) {
	wantOutput(t, presets(t), "kotlin", []File{
		{Name: "main.kt", Content: "fun main(args: Array<String>) { println(greet(args[0])) }\n"},
		{Name: "greet.kt", Content: "fun greet(who: String) = \"hi from kotlin, $who\"\n"},
	}, []string{"playbook"}, "hi from kotlin, playbook")
}

// Kotlin script entry: no compile step, runs via kotlinc -script with args.
func TestKotlinScript(t *testing.T) {
	wantOutput(t, presets(t), "kotlin", []File{
		{Name: "hello.kts", Content: "println(\"hi from kts \" + args.joinToString(\"|\"))\n"},
	}, []string{"x", "y"}, "hi from kts x|y")
}
