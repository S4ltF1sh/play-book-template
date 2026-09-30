package main

// Authoring subcommands. They run against the embedded content — the same
// build the playground serves — so `go run . <cmd>` always checks what is
// on disk right now:
//
//	playbook toolchains [list | verify [name...]]
//	playbook exercises verify [chNN[/exercise-id]...] [--only solution|starter]
//	playbook content lint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"playbook/internal/content"
	"playbook/internal/runner"
)

var subcommands = map[string]func([]string) int{
	"toolchains": toolchainsCmd,
	"exercises":  exercisesCmd,
	"content":    contentCmd,
}

// templateAppID is the app_id the template ships with. A playbook that
// keeps it shares its data dir — progress DB and $ENV_DIR — with the
// template demo and every other copy that forgot to change it.
const templateAppID = "playbook-template-demo"

// splitArgs lets flags come anywhere: it pulls them in front of the
// positionals. valueFlags names the flags that take a value.
func splitArgs(args []string, valueFlags ...string) (flags, pos []string) {
	takesValue := map[string]bool{}
	for _, f := range valueFlags {
		takesValue["-"+f], takesValue["--"+f] = true, true
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case !strings.HasPrefix(a, "-"):
			pos = append(pos, a)
		case takesValue[a] && i+1 < len(args):
			flags = append(flags, a, args[i+1])
			i++
		default:
			flags = append(flags, a)
		}
	}
	return flags, pos
}

// envFlags registers --data/--env-dir and returns a loader for the registry.
func envFlags(fset *flag.FlagSet) func() *runner.Registry {
	dataDir := fset.String("data", "", "directory for user data (default: OS config dir)")
	envDir := fset.String("env-dir", "", "override $ENV_DIR (default: $PLAYBOOK_ENV_DIR, else <data>/env)")
	return func() *runner.Registry {
		appID, _ := courseMeta()
		if appID == templateAppID {
			fmt.Fprintf(os.Stderr, "warning: app_id is still %q — set a unique app_id in content/course.json, or this playbook shares its data dir and $ENV_DIR with the template demo\n", templateAppID)
		}
		return loadToolchains(resolveDataDir(*dataDir, appID), *envDir)
	}
}

// toolchainsCmd implements `playbook toolchains [list|verify [name...]]`:
// the playbook-setup-env skill's way to check the environment with the
// exact definitions the playground uses.
func toolchainsCmd(args []string) int {
	fset := flag.NewFlagSet("toolchains", flag.ExitOnError)
	load := envFlags(fset)
	fset.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: playbook toolchains [--data DIR] [--env-dir DIR] [list | verify [name...]]")
		fset.PrintDefaults()
	}
	flags, pos := splitArgs(args, "data", "env-dir")
	_ = fset.Parse(flags)
	sub := ""
	if len(pos) > 0 {
		sub = pos[0]
	}
	reg := load()
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
	out, code, err := runPane(reg, "verify/"+name, name, tc.Scratch, nil, 3*time.Minute)
	if err != nil {
		return out, err
	}
	if code != 0 {
		return out, fmt.Errorf("exit %d", code)
	}
	return out, nil
}

// ansiRE matches terminal escape sequences; the UI strips them before
// showing output and matching checks (web/app.js ANSI_RE), so do we.
var ansiRE = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[@-Z\\-_]`)

// runPane runs one pane through the real runner, like the Run button: the
// same staging, environment and pty; output with escape sequences removed.
func runPane(reg *runner.Registry, pane, toolchain string, files []runner.File, args []string, timeout time.Duration) (string, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	sess := reg.NewSession(ctx)
	sess.Pane = pane
	defer sess.Close()

	var mu sync.Mutex
	var out strings.Builder
	collect := func(s string) { mu.Lock(); out.WriteString(s); mu.Unlock() }
	result := func() string { mu.Lock(); defer mu.Unlock(); return ansiRE.ReplaceAllString(out.String(), "") }
	exit := make(chan int, 1)
	if err := sess.Run(toolchain, files, args, collect, func(string) {}, func(c int) { exit <- c }); err != nil {
		return result(), -1, err
	}
	select {
	case code := <-exit:
		return result(), code, nil
	case <-ctx.Done():
		sess.Kill()
		return result(), -1, fmt.Errorf("timed out after %s (killed) — does it wait for stdin?", timeout)
	}
}

func indent(s string) string {
	return "    " + strings.ReplaceAll(strings.TrimRight(s, "\n"), "\n", "\n    ")
}

func loadContent() (*content.Content, *content.Manifest, error) {
	sub, err := fs.Sub(contentFS, "content")
	if err != nil {
		return nil, nil, err
	}
	c, err := content.Load(sub)
	if err != nil {
		return nil, nil, err
	}
	m, err := c.Manifest()
	return c, m, err
}

// matcher evaluates check patterns with JavaScript RegExp semantics, as the
// UI does: Go's regexp when the pattern compiles there (the common subset
// behaves the same), else node for JS-only syntax (lookaround, \1, ...).
type matcher struct{ node string }

func newMatcher() *matcher {
	node, _ := exec.LookPath("node")
	return &matcher{node}
}

func (m *matcher) test(pattern, text string) (bool, error) {
	if re, err := regexp.Compile(pattern); err == nil {
		return re.MatchString(text), nil
	}
	if m.node == "" {
		return false, fmt.Errorf("pattern uses JavaScript-only regex syntax; install node to evaluate it")
	}
	in, _ := json.Marshal([2]string{pattern, text})
	cmd := exec.Command(m.node, "-e", `let d="";process.stdin.on("data",c=>d+=c).on("end",()=>{const [p,t]=JSON.parse(d);try{process.stdout.write(new RegExp(p).test(t)?"1":"0")}catch(e){process.stdout.write("E"+e.message)}})`)
	cmd.Stdin = bytes.NewReader(in)
	out, err := cmd.Output()
	switch {
	case err != nil:
		return false, err
	case bytes.HasPrefix(out, []byte("E")):
		return false, errors.New(string(out[1:]))
	}
	return string(out) == "1", nil
}

// exercisesCmd implements `playbook exercises verify`: every exercise's
// panes run through the real runner — solution files in place of starter
// files for the solution run — and its checks are evaluated on the output
// exactly as the UI does. A solution must pass every check; a starter must
// fail at least one (otherwise the exercise needs no work).
func exercisesCmd(args []string) int {
	fset := flag.NewFlagSet("exercises", flag.ExitOnError)
	load := envFlags(fset)
	only := fset.String("only", "", `run only "solution" or "starter"`)
	verbose := fset.Bool("v", false, "print every pane's output, not only failures")
	fset.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: playbook exercises verify [chNN[/exercise-id]...] [--only solution|starter] [-v] [--data DIR] [--env-dir DIR]")
		fset.PrintDefaults()
	}
	flags, pos := splitArgs(args, "data", "env-dir", "only")
	_ = fset.Parse(flags)
	if len(pos) == 0 || pos[0] != "verify" || (*only != "" && *only != "solution" && *only != "starter") {
		fset.Usage()
		return 2
	}
	want := map[string]bool{}
	for _, p := range pos[1:] {
		want[strings.TrimSuffix(p, "/")] = true
	}
	c, m, err := loadContent()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	reg := load()
	mt := newMatcher()
	modes := []string{"solution", "starter"}
	if *only != "" {
		modes = []string{*only}
	}

	failed, ran := 0, 0
	for _, ch := range m.Chapters {
		exs, err := c.Exercises(ch.ID)
		if err != nil {
			continue // chapter without exercises.json
		}
		for _, ex := range exs {
			if len(want) > 0 && !want[ch.ID] && !want[ch.ID+"/"+ex.ID] {
				continue
			}
			for _, mode := range modes {
				if mode == "solution" && ex.Solution == nil {
					fmt.Printf("SKIP %s/%s solution (none)\n", ch.ID, ex.ID)
					continue
				}
				ran++
				if !verifyExercise(c, reg, mt, m, ch.ID, ex, mode, *verbose) {
					failed++
				}
			}
		}
	}
	if ran == 0 {
		fmt.Println("no exercises matched")
		return 1
	}
	fmt.Printf("\n%d run(s), %d failed\n", ran, failed)
	if failed > 0 {
		return 1
	}
	return 0
}

func verifyExercise(c *content.Content, reg *runner.Registry, mt *matcher, m *content.Manifest, ch string, ex content.Exercise, mode string, verbose bool) bool {
	id := ch + "/" + ex.ID
	sol := map[string]bool{}
	if mode == "solution" {
		for _, f := range ex.Solution.Files {
			sol[f] = true
		}
	}
	outputs := make([]string, len(ex.Panes))
	var problems []string
	for i, p := range ex.Panes {
		var files []runner.File
		for _, f := range p.Files {
			read := c.StarterFile
			if sol[f] {
				read = c.SolutionFile
			}
			b, err := read(ch, ex.ID, f)
			if err != nil {
				problems = append(problems, fmt.Sprintf("pane %d: missing file %s", i, f))
				continue
			}
			files = append(files, runner.File{Name: f, Content: string(b)})
		}
		tc := p.Toolchain
		if tc == "" {
			tc = m.DefaultToolchain
		}
		// same pane id as the UI, so it warms the same persistent workspace
		out, code, err := runPane(reg, fmt.Sprintf("%s/%d", id, i), tc, files, strings.Fields(p.DefaultArgs), 2*time.Minute)
		outputs[i] = out
		if err != nil {
			problems = append(problems, fmt.Sprintf("pane %d (%s): %v", i, tc, err))
		}
		if verbose || err != nil {
			fmt.Printf("---- %s %s pane %d (%s) exit %d\n%s\n", id, mode, i, tc, code, indent(out))
		}
	}
	passed := 0
	var failing []string
	for i, ck := range ex.Checks {
		ok := false
		if ck.Pane >= 0 && ck.Pane < len(outputs) {
			var err error
			if ok, err = mt.test(ck.Pattern, outputs[ck.Pane]); err != nil {
				problems = append(problems, fmt.Sprintf("check %d /%s/: %v", i, ck.Pattern, err))
			}
		} else {
			problems = append(problems, fmt.Sprintf("check %d: no pane %d", i, ck.Pane))
		}
		if ok {
			passed++
		} else {
			failing = append(failing, fmt.Sprintf("check %d (pane %d) /%s/", i, ck.Pane, ck.Pattern))
		}
	}
	total := len(ex.Checks)
	good := len(problems) == 0
	switch mode {
	case "solution":
		good = good && passed == total
	case "starter":
		good = good && (total == 0 || passed < total)
		if total > 0 && passed == total {
			problems = append(problems, "the starter already passes every check — the exercise needs no work")
		}
	}
	status := "PASS"
	if !good {
		status = "FAIL"
	}
	fmt.Printf("%s %s %s (%d/%d checks pass)\n", status, id, mode, passed, total)
	for _, p := range problems {
		fmt.Printf("    %s\n", p)
	}
	if !good && mode == "solution" {
		for _, f := range failing {
			fmt.Printf("    failing: %s\n", f)
		}
		if !verbose {
			for i, o := range outputs {
				fmt.Printf("    ---- pane %d output\n%s\n", i, indent(indent(o)))
			}
		}
	}
	return good
}

// contentCmd implements `playbook content lint`: the mechanical rules of
// playbook-generate/reference.md, checked over the whole content tree.
func contentCmd(args []string) int {
	if len(args) != 1 || args[0] != "lint" {
		fmt.Fprintln(os.Stderr, "usage: playbook content lint")
		return 2
	}
	c, m, err := loadContent()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	l := &linter{c: c, m: m, mt: newMatcher()}
	l.run()
	for _, w := range l.warnings {
		fmt.Println("WARN  " + w)
	}
	for _, e := range l.errors {
		fmt.Println("ERROR " + e)
	}
	fmt.Printf("%d error(s), %d warning(s)\n", len(l.errors), len(l.warnings))
	if len(l.errors) > 0 {
		return 1
	}
	return 0
}

type linter struct {
	c                *content.Content
	m                *content.Manifest
	mt               *matcher
	errors, warnings []string
}

func (l *linter) errf(f string, a ...any)  { l.errors = append(l.errors, fmt.Sprintf(f, a...)) }
func (l *linter) warnf(f string, a ...any) { l.warnings = append(l.warnings, fmt.Sprintf(f, a...)) }

func (l *linter) exists(p string) bool {
	_, err := fs.Stat(contentFS, "content/"+p)
	return err == nil
}

func (l *linter) read(p string) []byte {
	b, _ := fs.ReadFile(contentFS, "content/"+p)
	return b
}

// localized checks that a per-locale map has every locale filled in.
func (l *linter) localized(where string, v map[string]string) {
	for _, loc := range l.m.Locales {
		if strings.TrimSpace(v[loc]) == "" {
			l.errf("%s: missing %q text", where, loc)
		}
	}
}

// perLocale checks that a per-locale file exists (and parses, for JSON).
func (l *linter) perLocale(pattern string) {
	for _, loc := range l.m.Locales {
		p := fmt.Sprintf(pattern, loc)
		if !l.exists(p) {
			l.errf("%s: missing", p)
			continue
		}
		if strings.HasSuffix(p, ".json") && !json.Valid(l.read(p)) {
			l.errf("%s: invalid JSON", p)
		}
	}
}

func (l *linter) run() {
	m := l.m
	if m.AppID == templateAppID {
		l.warnf("course.json: app_id is still %q — set a unique one before anything else (it names the data dir and $ENV_DIR)", templateAppID)
	}
	if len(m.Locales) == 0 {
		l.errf("course.json: no locales")
	}
	defaults, _ := contentFS.ReadFile("content/toolchains.json")
	var tcs []struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(defaults, &tcs)
	defined := map[string]bool{}
	for _, t := range tcs {
		defined[t.Name] = true
	}
	if m.DefaultToolchain != "" && !defined[m.DefaultToolchain] {
		l.errf("course.json: default_toolchain %q is not defined in toolchains.json", m.DefaultToolchain)
	}
	chIDs := map[string]bool{}
	for _, ch := range m.Chapters {
		if chIDs[ch.ID] {
			l.errf("course.json: duplicate chapter id %s", ch.ID)
		}
		chIDs[ch.ID] = true
		l.localized("course.json "+ch.ID+" title", ch.Title)
		if ch.Status == "planned" {
			continue
		}
		l.chapter(ch, defined)
	}
	l.emoji()
}

func (l *linter) chapter(ch content.Chapter, defined map[string]bool) {
	dir := "chapters/" + ch.ID + "/"
	var exs []content.Exercise
	if ch.HasExercises || l.exists(dir+"exercises.json") {
		var err error
		if exs, err = l.c.Exercises(ch.ID); err != nil {
			l.errf("%sexercises.json: %v", dir, err)
		}
	}
	exByID := map[string]content.Exercise{}
	for _, ex := range exs {
		exByID[ex.ID] = ex
	}
	secIDs := map[string]bool{}
	for _, sec := range ch.Sections {
		where := fmt.Sprintf("course.json %s/%s", ch.ID, sec.ID)
		if secIDs[sec.ID] {
			l.errf("%s: duplicate section id", where)
		}
		secIDs[sec.ID] = true
		l.localized(where+" title", sec.Title)
		switch sec.Type {
		case "":
			l.perLocale(dir + "sections/" + sec.ID + ".%s.md")
		case "quiz":
			l.quiz(dir+"quizzes/"+sec.Quiz+".%s.json", ch.ID, sec.Quiz)
		case "exercise":
			if _, ok := exByID[sec.Exercise]; !ok {
				l.errf("%s: exercise %q not in %sexercises.json", where, sec.Exercise, dir)
			}
		default:
			l.errf("%s: unknown section type %q", where, sec.Type)
		}
	}
	if ch.HasSummary {
		l.perLocale(dir + "summary.%s.md")
		l.perLocale(dir + "glossary.%s.json")
	}
	if ch.HasQuiz {
		l.quiz(dir+"quiz.%s.json", ch.ID, "")
	}
	l.exercises(ch.ID, exs, defined)
}

func (l *linter) quiz(pattern, ch, id string) {
	l.perLocale(pattern)
	var ref *content.Quiz
	refLoc := ""
	for _, loc := range l.m.Locales {
		p := fmt.Sprintf(pattern, loc)
		if !l.exists(p) {
			continue
		}
		q, err := l.c.Quiz(ch, id, loc)
		if err != nil {
			l.errf("%s: %v", p, err)
			continue
		}
		if len(q.Questions) == 0 {
			l.errf("%s: no questions", p)
		}
		for _, qq := range q.Questions {
			where := p + " " + qq.ID
			if qq.Answer < 0 || qq.Answer >= len(qq.Choices) {
				l.errf("%s: answer %d out of range", where, qq.Answer)
				continue
			}
			if strings.TrimSpace(qq.Explanation) == "" {
				l.errf("%s: missing explanation", where)
			}
			// the right answer must not give itself away by being the longest
			correct := utf8.RuneCountInString(qq.Choices[qq.Answer])
			longest := true
			for i, ch := range qq.Choices {
				if i != qq.Answer && utf8.RuneCountInString(ch) >= correct {
					longest = false
				}
			}
			if longest && len(qq.Choices) > 1 {
				l.errf("%s: the correct choice is the longest one", where)
			}
		}
		if ref == nil {
			ref, refLoc = q, loc
			continue
		}
		if len(q.Questions) != len(ref.Questions) {
			l.errf("%s: %d questions, %s has %d", p, len(q.Questions), refLoc, len(ref.Questions))
			continue
		}
		for i, qq := range q.Questions {
			r := ref.Questions[i]
			if qq.ID != r.ID || qq.Answer != r.Answer || len(qq.Choices) != len(r.Choices) {
				l.errf("%s: question %d (%s) differs from %s (id/answer/choice count must match)", p, i+1, qq.ID, refLoc)
			}
		}
	}
}

func (l *linter) exercises(ch string, exs []content.Exercise, defined map[string]bool) {
	dir := "chapters/" + ch + "/"
	flatUsers := map[string][]string{} // chapter-wide starter file -> exercises using it
	seen := map[string]bool{}
	for _, ex := range exs {
		where := fmt.Sprintf("%sexercises.json %s", dir, ex.ID)
		if seen[ex.ID] {
			l.errf("%s: duplicate exercise id", where)
		}
		seen[ex.ID] = true
		l.localized(where+" title", ex.Title)
		l.localized(where+" brief", ex.Brief)
		l.localized(where+" tasks", ex.Tasks)
		if len(ex.Panes) == 0 {
			l.errf("%s: no panes", where)
		}
		for i, p := range ex.Panes {
			l.localized(fmt.Sprintf("%s pane %d name", where, i), p.Name)
			if p.Toolchain != "" && !defined[p.Toolchain] {
				l.errf("%s pane %d: toolchain %q not defined in toolchains.json", where, i, p.Toolchain)
			}
			if len(p.Files) == 0 {
				l.errf("%s pane %d: no files", where, i)
			}
			for _, f := range p.Files {
				switch {
				case l.exists(dir + "starter/" + ex.ID + "/" + f):
				case l.exists(dir + "starter/" + f):
					flatUsers[f] = append(flatUsers[f], ex.ID)
				default:
					l.errf("%s: starter file %s missing (starter/%s/%s or starter/%s)", where, f, ex.ID, f, f)
				}
			}
		}
		for i, ck := range ex.Checks {
			cw := fmt.Sprintf("%s check %d", where, i)
			l.localized(cw+" label", ck.Label)
			if ck.Pane < 0 || ck.Pane >= len(ex.Panes) {
				l.errf("%s: pane %d does not exist", cw, ck.Pane)
			}
			if _, err := l.mt.test(ck.Pattern, ""); err != nil {
				l.errf("%s: pattern /%s/: %v", cw, ck.Pattern, err)
			}
		}
		if ex.Solution != nil {
			l.localized(where+" solution notes", ex.Solution.Notes)
			for _, f := range ex.Solution.Files {
				if !l.exists(dir+"solution/"+ex.ID+"/"+f) && !l.exists(dir+"solution/"+f) {
					l.errf("%s: solution file %s missing (solution/%s/%s or solution/%s)", where, f, ex.ID, f, f)
				}
			}
		}
	}
	for f, users := range flatUsers {
		if len(users) > 1 {
			l.warnf("%sstarter/%s is shared by exercises %s — fine only if they really start from the same file; otherwise use starter/<exercise-id>/%s", dir, f, strings.Join(users, ", "), f)
		}
	}
}

// emoji flags emoji in content text: the design uses line icons only.
func (l *linter) emoji() {
	isEmoji := func(r rune) bool {
		return (r >= 0x1F000 && r <= 0x1FAFF) || (r >= 0x2600 && r <= 0x27BF) || r == 0xFE0F || (r >= 0x2B00 && r <= 0x2BFF)
	}
	_ = fs.WalkDir(contentFS, "content/chapters", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || (!strings.HasSuffix(p, ".md") && !strings.HasSuffix(p, ".json")) {
			return nil
		}
		for i, line := range strings.Split(string(l.read(strings.TrimPrefix(p, "content/"))), "\n") {
			for _, r := range line {
				if isEmoji(r) {
					l.errf("%s:%d: emoji %q (use words; the UI has line icons)", strings.TrimPrefix(p, "content/"), i+1, string(r))
					break
				}
			}
		}
		return nil
	})
}
