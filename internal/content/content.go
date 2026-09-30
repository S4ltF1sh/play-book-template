// Package content serves the embedded course material with locale fallback.
package content

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

type Content struct {
	fsys fs.FS
	md   goldmark.Markdown
	// Locales in fallback order, e.g. ["vi", "en"]: a request for locale L
	// tries L first, then the others in this order.
	Locales []string
}

type Manifest struct {
	// AppID names the user-data directory (progress DB); set once, never rename.
	AppID string `json:"app_id"`
	// Brand is the short name shown in the topbar and breadcrumbs.
	Brand map[string]string `json:"brand"`
	// BrandIcon names a built-in line icon for the topbar + favicon
	// (see ICONS in web/app.js); "" means the default, "none" hides it.
	BrandIcon string `json:"brand_icon,omitempty"`
	// Tagline/Subtitle fill the home hero.
	Tagline  map[string]string `json:"tagline"`
	Subtitle map[string]string `json:"subtitle"`
	// DefaultToolchain is used by scratch panes and panes with no toolchain
	// of their own: "c" | "python" | "node" | "kotlin" | "java".
	DefaultToolchain string    `json:"default_toolchain"`
	Locales          []string  `json:"locales"`
	DefaultLocale    string    `json:"default_locale"`
	Chapters         []Chapter `json:"chapters"`
}

type Chapter struct {
	ID           string            `json:"id"`
	Slug         string            `json:"slug"`
	Title        map[string]string `json:"title"`
	Status       string            `json:"status"` // "ready" | "planned"
	Sections     []Section         `json:"sections"`
	HasQuiz      bool              `json:"has_quiz"`
	HasExercises bool              `json:"has_exercises"`
	HasSummary   bool              `json:"has_summary"`
}

type Section struct {
	ID       string            `json:"id"`
	Title    map[string]string `json:"title"`
	Type     string            `json:"type,omitempty"`     // "" (reading) | "exercise" | "quiz"
	Exercise string            `json:"exercise,omitempty"` // exercise id for type "exercise"
	Quiz     string            `json:"quiz,omitempty"`     // quiz id for type "quiz"
}

func Load(fsys fs.FS) (*Content, error) {
	c := &Content{
		fsys: fsys,
		md: goldmark.New(
			goldmark.WithExtensions(extension.GFM),
			goldmark.WithRendererOptions(html.WithUnsafe()), // our own content; allows inline <img>/<div>
		),
	}
	m, err := c.Manifest()
	if err != nil {
		return nil, err
	}
	c.Locales = m.Locales
	return c, nil
}

func (c *Content) Manifest() (*Manifest, error) {
	b, err := fs.ReadFile(c.fsys, "course.json")
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("course.json: %w", err)
	}
	return &m, nil
}

// localized reads pattern (with %s for locale) trying lang first, then the
// remaining locales in manifest order.
func (c *Content) localized(pattern, lang string) ([]byte, error) {
	tried := []string{lang}
	for _, l := range c.Locales {
		if l != lang {
			tried = append(tried, l)
		}
	}
	var lastErr error
	for _, l := range tried {
		b, err := fs.ReadFile(c.fsys, fmt.Sprintf(pattern, l))
		if err == nil {
			return b, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (c *Content) render(md []byte) (string, error) {
	var buf bytes.Buffer
	if err := c.md.Convert(md, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (c *Content) SectionHTML(ch, id, lang string) (string, error) {
	b, err := c.localized("chapters/"+ch+"/sections/"+id+".%s.md", lang)
	if err != nil {
		return "", err
	}
	return c.render(b)
}

func (c *Content) SummaryHTML(ch, lang string) (string, error) {
	b, err := c.localized("chapters/"+ch+"/summary.%s.md", lang)
	if err != nil {
		return "", err
	}
	return c.render(b)
}

func (c *Content) GlossaryJSON(ch, lang string) ([]byte, error) {
	return c.localized("chapters/"+ch+"/glossary.%s.json", lang)
}

type QuizQuestion struct {
	ID          string   `json:"id"`
	Prompt      string   `json:"prompt"`
	Choices     []string `json:"choices"`
	Answer      int      `json:"answer"`
	Explanation string   `json:"explanation"`
}

type Quiz struct {
	ID        string         `json:"id"`
	Questions []QuizQuestion `json:"questions"`
}

// Quiz loads the chapter-end quiz (id "") or a named inline quiz stored at
// chapters/<ch>/quizzes/<id>.<lang>.json.
func (c *Content) Quiz(ch, id, lang string) (*Quiz, error) {
	pattern := "chapters/" + ch + "/quiz.%s.json"
	if id != "" {
		pattern = "chapters/" + ch + "/quizzes/" + id + ".%s.json"
	}
	b, err := c.localized(pattern, lang)
	if err != nil {
		return nil, err
	}
	var q Quiz
	if err := json.Unmarshal(b, &q); err != nil {
		return nil, fmt.Errorf("quiz %s/%s: %w", ch, id, err)
	}
	return &q, nil
}

type ExercisePane struct {
	Name  map[string]string `json:"name"`
	Files []string          `json:"files"`
	// Toolchain overrides the manifest's default_toolchain for this pane.
	// Empty = inferred from the first file's extension by the runner.
	Toolchain   string `json:"toolchain,omitempty"`
	DefaultArgs string `json:"default_args"`
}

type ExerciseCheck struct {
	Pane    int               `json:"pane"`    // pane index whose output is matched
	Pattern string            `json:"pattern"` // regex tested against accumulated output
	Label   map[string]string `json:"label"`
}

// ExerciseSolution is the sample solution shown behind a disclosure in the
// lesson pane. Files list names under chapters/<ch>/solution/.
type ExerciseSolution struct {
	Notes map[string]string `json:"notes"` // per-locale explanation text
	Files []string          `json:"files"`
}

type Exercise struct {
	ID       string            `json:"id"`
	Title    map[string]string `json:"title"`
	Brief    map[string]string `json:"brief"` // markdown, rendered client-side as text
	Tasks    map[string]string `json:"tasks"` // markdown checklist
	Panes    []ExercisePane    `json:"panes"`
	Checks   []ExerciseCheck   `json:"checks"`
	Solution *ExerciseSolution `json:"solution,omitempty"`
}

func (c *Content) Exercises(ch string) ([]Exercise, error) {
	b, err := fs.ReadFile(c.fsys, "chapters/"+ch+"/exercises.json")
	if err != nil {
		return nil, err
	}
	var ex []Exercise
	if err := json.Unmarshal(b, &ex); err != nil {
		return nil, fmt.Errorf("exercises %s: %w", ch, err)
	}
	return ex, nil
}

func (c *Content) StarterFile(ch, name string) ([]byte, error) {
	return fs.ReadFile(c.fsys, "chapters/"+ch+"/starter/"+name)
}

func (c *Content) SolutionFile(ch, name string) ([]byte, error) {
	return fs.ReadFile(c.fsys, "chapters/"+ch+"/solution/"+name)
}

func (c *Content) Asset(ch, name string) ([]byte, error) {
	return fs.ReadFile(c.fsys, "chapters/"+ch+"/assets/"+name)
}

// RenderMarkdown renders arbitrary course markdown (used for briefs/tasks).
func (c *Content) RenderMarkdown(md string) (string, error) {
	return c.render([]byte(md))
}
