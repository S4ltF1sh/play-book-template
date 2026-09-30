// Package runner compiles and runs playground programs locally, streaming
// their output through a pty so stdio is unbuffered and interactive.
//
// The runner knows no languages. Every toolchain is data — an entry of
// content/toolchains.json with shell commands for build and run plus rules
// for staging the pane's files — so a playbook can use anything its
// environment provides (make, gradle, cargo, a venv, ...). Setting that
// environment up is the playbook-setup-env skill's job, not the runner's.
package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

const (
	runTimeout   = 5 * time.Minute
	buildTimeout = 5 * time.Minute
)

type File struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// Toolchain is one entry of content/toolchains.json.
type Toolchain struct {
	Name  string `json:"name"`
	Label string `json:"label,omitempty"`
	// Detect lists what must exist for the toolchain to count as installed.
	// Bare names are executables searched in $ENV_DIR/bin, then PATH; a
	// value containing "/" is a file or directory path that must exist
	// (after $VAR expansion) — e.g. "$ENV_DIR/venv" or a prefetched cache,
	// so a toolchain whose private environment never got set up shows as
	// missing instead of failing at build time.
	Detect []string `json:"detect"`
	// Sources are glob patterns (matched against the file's base name) of
	// the files that get compiled or executed. The first source listed in a
	// pane is the entry point ($ENTRY); all of them together are $SRCS.
	Sources []string `json:"sources"`
	// Stage are patterns of extra files written to the workdir but never
	// treated as sources: headers, data files, build manifests. A pane file
	// matching neither list is dropped.
	Stage []string `json:"stage,omitempty"`
	// Layout maps a pattern to the subdirectory its files are staged into,
	// e.g. {"*.rs": "src"} for Cargo; "." keeps a file at the root. The most
	// specific pattern wins: an exact name beats a glob ({"build.rs": "."}
	// overrides "*.rs"), then the longer pattern. Unmatched files go to the
	// workdir root.
	Layout map[string]string `json:"layout,omitempty"`
	// Workspace is "temp" (default: a fresh directory per Run) or
	// "persistent" (reused across Runs of the same pane, so incremental build
	// tools skip unchanged work; files a previous Run staged but this one
	// doesn't are removed).
	Workspace string `json:"workspace,omitempty"`
	// Env adds variables to the build/run environment; values may reference
	// $ENV_DIR, $PATH and any other variable.
	Env map[string]string `json:"env,omitempty"`
	// Build is an optional sh command run before Run (compile step).
	Build string `json:"build,omitempty"`
	// Run is the sh command that starts the program; the pane's args arrive
	// as "$@".
	Run string `json:"run"`
	// Scratch is the free-playground starter; its first file is the entry.
	// `playbook toolchains verify` runs it as the toolchain's smoke test.
	Scratch []File `json:"scratch,omitempty"`
}

// Registry holds the toolchain definitions of one playbook.
type Registry struct {
	list   []*Toolchain
	byName map[string]*Toolchain
	// envDir is the playbook's private environment ($ENV_DIR): venvs,
	// vendored libraries, extra binaries in $ENV_DIR/bin.
	envDir string
	// workDir holds persistent workspaces.
	workDir string

	mu     sync.Mutex
	claims map[string]*Session // persistent workspace dir -> holder
}

// Load parses toolchains.json (a JSON array; order = extension-inference
// priority) and validates every entry.
func Load(data []byte, envDir, workDir string) (*Registry, error) {
	var list []*Toolchain
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("toolchains.json: %w", err)
	}
	r := &Registry{byName: map[string]*Toolchain{}, envDir: envDir, workDir: workDir, claims: map[string]*Session{}}
	for i, tc := range list {
		where := fmt.Sprintf("toolchains.json[%d] (%q)", i, tc.Name)
		switch {
		case tc.Name == "":
			return nil, fmt.Errorf("%s: missing name", where)
		case r.byName[tc.Name] != nil:
			return nil, fmt.Errorf("%s: duplicate name", where)
		case tc.Run == "":
			return nil, fmt.Errorf("%s: missing run command", where)
		case len(tc.Sources) == 0:
			return nil, fmt.Errorf("%s: no source patterns", where)
		case tc.Workspace != "" && tc.Workspace != "temp" && tc.Workspace != "persistent":
			return nil, fmt.Errorf("%s: workspace must be \"temp\" or \"persistent\"", where)
		}
		for _, p := range append(append([]string{}, tc.Sources...), tc.Stage...) {
			if _, err := path.Match(p, ""); err != nil {
				return nil, fmt.Errorf("%s: bad pattern %q", where, p)
			}
		}
		for p, dir := range tc.Layout {
			if _, err := path.Match(p, ""); err != nil {
				return nil, fmt.Errorf("%s: bad layout pattern %q", where, p)
			}
			if !filepath.IsLocal(dir) {
				return nil, fmt.Errorf("%s: layout dir %q must be a relative path inside the workdir", where, dir)
			}
		}
		r.list = append(r.list, tc)
		r.byName[tc.Name] = tc
	}
	if len(r.list) == 0 {
		return nil, errors.New("toolchains.json: no toolchains defined")
	}
	return r, nil
}

func (r *Registry) EnvDir() string           { return r.envDir }
func (r *Registry) Toolchains() []*Toolchain { return r.list }

func matchAny(patterns []string, name string) bool {
	base := path.Base(name)
	for _, p := range patterns {
		if ok, _ := path.Match(p, base); ok {
			return true
		}
	}
	return false
}

// getenv resolves $VAR references in definitions: ENV_DIR first, then the
// server's own environment.
func (r *Registry) getenv(k string) string {
	if k == "ENV_DIR" {
		return r.envDir
	}
	return os.Getenv(k)
}

func isExecutable(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir() && st.Mode()&0o111 != 0
}

// missing returns the first Detect entry that can't be found, or "".
func (r *Registry) missing(tc *Toolchain) string {
	for _, bin := range tc.Detect {
		b := os.Expand(bin, r.getenv)
		if strings.Contains(bin, "/") {
			if _, err := os.Stat(b); err != nil {
				return bin
			}
			continue
		}
		if isExecutable(filepath.Join(r.envDir, "bin", b)) {
			continue
		}
		if _, err := exec.LookPath(b); err != nil {
			return bin
		}
	}
	return ""
}

// Resolve picks the toolchain: explicit name first, else the first
// definition whose sources match the first file, else the first definition.
func (r *Registry) Resolve(name string, files []File) (*Toolchain, error) {
	if name == "" && len(files) > 0 {
		for _, tc := range r.list {
			if matchAny(tc.Sources, files[0].Name) {
				name = tc.Name
				break
			}
		}
	}
	if name == "" {
		name = r.list[0].Name
	}
	tc, ok := r.byName[name]
	if !ok {
		return nil, fmt.Errorf("unknown toolchain %q — define it in content/toolchains.json / toolchain %q chưa được định nghĩa", name, name)
	}
	if bin := r.missing(tc); bin != "" {
		return nil, fmt.Errorf("toolchain %q is not installed (missing %q) — run the playbook-setup-env skill / toolchain %q chưa được cài trên máy", tc.Name, bin, tc.Name)
	}
	return tc, nil
}

// Available reports which defined toolchains are installed on this machine.
func (r *Registry) Available() map[string]bool {
	out := map[string]bool{}
	for _, tc := range r.list {
		out[tc.Name] = r.missing(tc) == ""
	}
	return out
}

// PublicToolchain is what the frontend needs: no commands.
type PublicToolchain struct {
	Name    string `json:"name"`
	Label   string `json:"label,omitempty"`
	Scratch []File `json:"scratch,omitempty"`
}

func (r *Registry) Public() []PublicToolchain {
	out := make([]PublicToolchain, 0, len(r.list))
	for _, tc := range r.list {
		out = append(out, PublicToolchain{tc.Name, tc.Label, tc.Scratch})
	}
	return out
}

// environ builds the build/run environment: the server's own, plus
// $ENTRY/$SRCS/$ENV_DIR, $ENV_DIR/bin in front of PATH, then the
// toolchain's Env (expanded against everything before it).
func (r *Registry) environ(tc *Toolchain, entry string, srcs []string) []string {
	env := map[string]string{}
	var order []string
	set := func(k, v string) {
		if _, ok := env[k]; !ok {
			order = append(order, k)
		}
		env[k] = v
	}
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok {
			set(k, v)
		}
	}
	set("ENV_DIR", r.envDir)
	set("ENTRY", entry)
	set("SRCS", strings.Join(srcs, " "))
	set("PATH", filepath.Join(r.envDir, "bin")+string(os.PathListSeparator)+env["PATH"])
	keys := make([]string, 0, len(tc.Env))
	for k := range tc.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		set(k, os.Expand(tc.Env[k], func(v string) string { return env[v] }))
	}
	out := make([]string, 0, len(order))
	for _, k := range order {
		out = append(out, k+"="+env[k])
	}
	return out
}

// shell runs a definition command; args become "$@".
func shell(ctx context.Context, command string, args []string) *exec.Cmd {
	return exec.CommandContext(ctx, "/bin/sh", append([]string{"-c", command, "sh"}, args...)...)
}

// layoutDir returns where a file is staged: the most specific matching
// pattern wins — an exact name before any glob, then the longer pattern,
// then lexical order, so the result is deterministic.
func layoutDir(tc *Toolchain, name string) string {
	isGlob := func(p string) bool { return strings.ContainsAny(p, "*?[\\") }
	pats := make([]string, 0, len(tc.Layout))
	for p := range tc.Layout {
		pats = append(pats, p)
	}
	sort.Slice(pats, func(i, j int) bool {
		a, b := pats[i], pats[j]
		if isGlob(a) != isGlob(b) {
			return !isGlob(a)
		}
		if len(a) != len(b) {
			return len(a) > len(b)
		}
		return a < b
	})
	for _, p := range pats {
		if matchAny([]string{p}, name) {
			if d := tc.Layout[p]; d != "." {
				return d
			}
			return ""
		}
	}
	return ""
}

// claim reserves a persistent workspace for s; false if another session
// holds it (that pane then falls back to a temp dir).
func (r *Registry) claim(dir string, s *Session) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok := r.claims[dir]; ok && h != s {
		return false
	}
	r.claims[dir] = s
	return true
}

func (r *Registry) release(s *Session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for dir, h := range r.claims {
		if h == s {
			delete(r.claims, dir)
		}
	}
}

// Session is one playground pane: at most one running program at a time.
type Session struct {
	reg  *Registry
	ctx  context.Context
	mu   sync.Mutex
	cmd  *exec.Cmd
	ptmx *os.File
	// Pane identifies the pane across Runs (e.g. "ch02/buffers-lab/0",
	// "scratch/1"). It keys persistent workspaces, so two exercises that
	// happen to use the same file names don't share a build directory;
	// without it the key is the set of staged file names.
	Pane string
}

// NewSession starts a pane session; cancelling ctx aborts a running build.
func (r *Registry) NewSession(ctx context.Context) *Session {
	return &Session{reg: r, ctx: ctx}
}

type outWriter struct{ f func(string) }

func (w *outWriter) Write(p []byte) (int, error) { w.f(string(p)); return len(p), nil }

// stagedList records, inside a persistent workspace, which files the last
// Run staged, so files dropped from the pane since then can be removed.
const stagedList = ".playbook-staged"

// workspace returns the directory to stage into and a cleanup func.
func (s *Session) workspace(tc *Toolchain, names []string) (string, func(), error) {
	if tc.Workspace == "persistent" {
		// one directory per pane; clients that send no pane id get one per
		// set of staged file names
		key := "pane\x00" + s.Pane
		if s.Pane == "" {
			sorted := append([]string{}, names...)
			sort.Strings(sorted)
			key = "files\x00" + strings.Join(sorted, "\x00")
		}
		sum := sha256.Sum256([]byte(key))
		dir := filepath.Join(s.reg.workDir, tc.Name+"-"+hex.EncodeToString(sum[:6]))
		if s.reg.claim(dir, s) {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return "", nil, err
			}
			return dir, func() {}, nil
		}
	}
	dir, err := os.MkdirTemp("", "playbook-run-*")
	if err != nil {
		return "", nil, err
	}
	return dir, func() { os.RemoveAll(dir) }, nil
}

// pruneStaged removes files a previous Run staged into dir that the current
// plan no longer contains, then records the current plan.
func pruneStaged(dir string, names []string) error {
	keep := map[string]bool{}
	for _, n := range names {
		keep[n] = true
	}
	if old, err := os.ReadFile(filepath.Join(dir, stagedList)); err == nil {
		for _, n := range strings.Split(string(old), "\n") {
			if n != "" && !keep[n] && filepath.IsLocal(n) {
				_ = os.Remove(filepath.Join(dir, filepath.FromSlash(n)))
			}
		}
	}
	return os.WriteFile(filepath.Join(dir, stagedList), []byte(strings.Join(names, "\n")+"\n"), 0o644)
}

// writeIfChanged skips identical content so mtime-based build tools
// (make, cargo) don't rebuild untouched files in a persistent workspace.
func writeIfChanged(p string, content []byte) error {
	if old, err := os.ReadFile(p); err == nil && string(old) == string(content) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, content, 0o644)
}

// Run stages files, runs the build command if any, and starts the program.
// Events are delivered via the callbacks; onExit fires exactly once per
// successful start.
func (s *Session) Run(toolchain string, files []File, args []string, onOut func(string), onStatus func(string), onExit func(int)) error {
	s.Kill()

	s.mu.Lock()
	defer s.mu.Unlock()

	tc, err := s.reg.Resolve(toolchain, files)
	if err != nil {
		return err
	}

	type staged struct {
		path    string
		content string
	}
	var plan []staged
	var names, srcs []string
	entry := ""
	for _, f := range files {
		name := filepath.ToSlash(filepath.Clean(f.Name))
		if !filepath.IsLocal(name) {
			return fmt.Errorf("invalid file name %q / tên file không hợp lệ", f.Name)
		}
		isSrc := matchAny(tc.Sources, name)
		if !isSrc && !matchAny(tc.Stage, name) {
			continue
		}
		p := path.Join(filepath.ToSlash(layoutDir(tc, name)), name)
		plan = append(plan, staged{p, f.Content})
		names = append(names, p)
		if isSrc {
			if entry == "" {
				entry = p // first source file listed = entry point
			}
			srcs = append(srcs, p)
		}
	}
	if entry == "" {
		return fmt.Errorf("no %s source file to run / không có file nguồn %s nào để chạy", tc.Name, tc.Name)
	}

	dir, cleanup, err := s.workspace(tc, names)
	if err != nil {
		return err
	}
	if tc.Workspace == "persistent" {
		if err := pruneStaged(dir, names); err != nil {
			cleanup()
			return err
		}
	}
	for _, f := range plan {
		if err := writeIfChanged(filepath.Join(dir, filepath.FromSlash(f.path)), []byte(f.content)); err != nil {
			cleanup()
			return err
		}
	}
	env := s.reg.environ(tc, entry, srcs)

	if tc.Build != "" {
		onStatus("compiling")
		ctx, cancel := context.WithTimeout(s.ctx, buildTimeout)
		bc := shell(ctx, tc.Build, nil)
		bc.Dir, bc.Env = dir, env
		out := &outWriter{onOut}
		bc.Stdout, bc.Stderr = out, out
		// own process group, so a timeout also kills what the build forked
		bc.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		bc.Cancel = func() error { return syscall.Kill(-bc.Process.Pid, syscall.SIGKILL) }
		err := bc.Run()
		timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
		cancel()
		if err != nil {
			cleanup()
			if timedOut {
				return fmt.Errorf("build timed out after %s / biên dịch quá thời gian", buildTimeout)
			}
			return fmt.Errorf("compile failed / biên dịch thất bại")
		}
	}

	cmd := shell(context.Background(), tc.Run, args)
	cmd.Dir, cmd.Env = dir, env
	ptmx, err := pty.Start(cmd) // Setsid+Setctty: prog leads its own process group
	if err != nil {
		cleanup()
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
		cleanup()
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

// Close kills the program and frees the session's persistent workspaces.
func (s *Session) Close() {
	s.Kill()
	s.reg.release(s)
}
