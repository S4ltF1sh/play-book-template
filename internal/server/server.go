// Package server wires the HTTP API, static frontend, and playground WebSocket.
package server

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/coder/websocket"

	"playbook/internal/content"
	"playbook/internal/runner"
	"playbook/internal/store"
)

type Server struct {
	mux        *http.ServeMux
	content    *content.Content
	db         *store.Store
	toolchains *runner.Registry
}

func New(contentFS, webFS fs.FS, db *store.Store, toolchains *runner.Registry) *Server {
	c, err := content.Load(contentFS)
	if err != nil {
		log.Fatalf("cannot load course content: %v", err)
	}
	s := &Server{mux: http.NewServeMux(), content: c, db: db, toolchains: toolchains}

	// no-cache so a rebuilt binary never serves stale embedded assets
	fileSrv := http.FileServerFS(webFS)
	s.mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		fileSrv.ServeHTTP(w, r)
	}))
	s.mux.HandleFunc("GET /api/course", s.handleCourse)
	s.mux.HandleFunc("GET /api/section/{ch}/{id}", s.handleSection)
	s.mux.HandleFunc("POST /api/progress", s.handleProgress)
	s.mux.HandleFunc("GET /api/chapter/{ch}/summary", s.handleSummary)
	s.mux.HandleFunc("GET /api/chapter/{ch}/glossary", s.handleGlossary)
	s.mux.HandleFunc("GET /api/chapter/{ch}/quiz", s.handleQuiz)
	s.mux.HandleFunc("GET /api/chapter/{ch}/quiz/{id}", s.handleQuiz)
	s.mux.HandleFunc("POST /api/quiz/{ch}/attempt", s.handleQuizAttempt)
	s.mux.HandleFunc("POST /api/quiz/{ch}/{id}/attempt", s.handleQuizAttempt)
	s.mux.HandleFunc("GET /api/quiz/{ch}/attempts", s.handleQuizAttempts)
	s.mux.HandleFunc("GET /api/exercises/{ch}", s.handleExercises)
	s.mux.HandleFunc("POST /api/settings", s.handleSettings)
	s.mux.HandleFunc("GET /assets/{ch}/{name}", s.handleAsset)
	s.mux.HandleFunc("GET /ws/run", s.handleRun)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func httpErr(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)
	writeJSON(w, map[string]string{"error": msg})
}

func (s *Server) lang(r *http.Request) string {
	if l := r.URL.Query().Get("lang"); l != "" {
		return l
	}
	return s.db.Setting("locale", "vi")
}

// --- course / reader ---

func (s *Server) handleCourse(w http.ResponseWriter, r *http.Request) {
	m, err := s.content.Manifest()
	if err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	prog, err := s.db.Progress()
	if err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{
		"course":     m,
		"progress":   prog,
		"locale":     s.db.Setting("locale", m.DefaultLocale),
		"toolchains": s.toolchains.Available(),
		// name/label/scratch per definition (no commands) for the scratch pane
		"toolchain_defs": s.toolchains.Public(),
	})
}

func (s *Server) handleSection(w http.ResponseWriter, r *http.Request) {
	html, err := s.content.SectionHTML(r.PathValue("ch"), r.PathValue("id"), s.lang(r))
	if err != nil {
		httpErr(w, 404, "section not found")
		return
	}
	writeJSON(w, map[string]string{"html": html})
}

func (s *Server) handleProgress(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SectionID string `json:"section_id"`
		Status    string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SectionID == "" {
		httpErr(w, 400, "bad request")
		return
	}
	if err := s.db.SetProgress(req.SectionID, req.Status); err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	html, err := s.content.SummaryHTML(r.PathValue("ch"), s.lang(r))
	if err != nil {
		httpErr(w, 404, "summary not found")
		return
	}
	writeJSON(w, map[string]string{"html": html})
}

func (s *Server) handleGlossary(w http.ResponseWriter, r *http.Request) {
	b, err := s.content.GlossaryJSON(r.PathValue("ch"), s.lang(r))
	if err != nil {
		httpErr(w, 404, "glossary not found")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(b)
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	var req struct{ Locale string `json:"locale"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Locale == "" {
		httpErr(w, 400, "bad request")
		return
	}
	if err := s.db.SetSetting("locale", req.Locale); err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleAsset(w http.ResponseWriter, r *http.Request) {
	b, err := s.content.Asset(r.PathValue("ch"), r.PathValue("name"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if strings.HasSuffix(r.PathValue("name"), ".svg") {
		w.Header().Set("Content-Type", "image/svg+xml")
	}
	_, _ = w.Write(b)
}

// --- quiz ---

func (s *Server) handleQuiz(w http.ResponseWriter, r *http.Request) {
	q, err := s.content.Quiz(r.PathValue("ch"), r.PathValue("id"), s.lang(r))
	if err != nil {
		httpErr(w, 404, "quiz not found")
		return
	}
	// strip answers before sending to the client
	type pubQ struct {
		ID      string   `json:"id"`
		Prompt  string   `json:"prompt"`
		Choices []string `json:"choices"`
	}
	pub := make([]pubQ, len(q.Questions))
	for i, qq := range q.Questions {
		pub[i] = pubQ{qq.ID, qq.Prompt, qq.Choices}
	}
	writeJSON(w, map[string]any{"id": q.ID, "questions": pub})
}

func (s *Server) handleQuizAttempt(w http.ResponseWriter, r *http.Request) {
	ch, id := r.PathValue("ch"), r.PathValue("id")
	var req struct {
		Answers map[string]int `json:"answers"` // question id -> chosen index
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpErr(w, 400, "bad request")
		return
	}
	q, err := s.content.Quiz(ch, id, s.lang(r))
	if err != nil {
		httpErr(w, 404, "quiz not found")
		return
	}
	type result struct {
		ID          string `json:"id"`
		Chosen      int    `json:"chosen"`
		Correct     int    `json:"correct"`
		IsCorrect   bool   `json:"is_correct"`
		Explanation string `json:"explanation"`
	}
	results := make([]result, len(q.Questions))
	score := 0
	for i, qq := range q.Questions {
		chosen, ok := req.Answers[qq.ID]
		if !ok {
			chosen = -1
		}
		correct := chosen == qq.Answer
		if correct {
			score++
		}
		results[i] = result{qq.ID, chosen, qq.Answer, correct, qq.Explanation}
	}
	raw, _ := json.Marshal(results)
	key := ch
	if id != "" {
		key = ch + "/" + id
	}
	if err := s.db.AddQuizAttempt(key, score, len(q.Questions), string(raw)); err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"score": score, "total": len(q.Questions), "results": results})
}

func (s *Server) handleQuizAttempts(w http.ResponseWriter, r *http.Request) {
	at, err := s.db.QuizAttempts(r.PathValue("ch"))
	if err != nil {
		httpErr(w, 500, err.Error())
		return
	}
	writeJSON(w, at)
}

// --- exercises / playground ---

func (s *Server) handleExercises(w http.ResponseWriter, r *http.Request) {
	ch := r.PathValue("ch")
	exs, err := s.content.Exercises(ch)
	if err != nil {
		httpErr(w, 404, "no exercises")
		return
	}
	// bundle starter + solution file contents so the client has everything in one call
	files := map[string]string{}
	solutionFiles := map[string]string{}
	for _, ex := range exs {
		for _, p := range ex.Panes {
			for _, f := range p.Files {
				if _, done := files[f]; done {
					continue
				}
				b, err := s.content.StarterFile(ch, f)
				if err == nil {
					files[f] = string(b)
				}
			}
		}
		if ex.Solution != nil {
			for _, f := range ex.Solution.Files {
				if _, done := solutionFiles[f]; done {
					continue
				}
				b, err := s.content.SolutionFile(ch, f)
				if err == nil {
					solutionFiles[f] = string(b)
				}
			}
		}
	}
	writeJSON(w, map[string]any{"exercises": exs, "files": files, "solution_files": solutionFiles})
}

type wsIn struct {
	Op        string        `json:"op"` // run | stdin | kill
	Toolchain string        `json:"toolchain"`
	Files     []runner.File `json:"files"`
	Args      []string      `json:"args"`
	Data      string        `json:"data"`
}

type wsOut struct {
	Type string `json:"type"` // out | status | exit | error
	Data string `json:"data"`
	Code int    `json:"code"`
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	ctx := r.Context()
	sess := s.toolchains.NewSession(ctx)
	defer sess.Close()

	var wmu sync.Mutex
	send := func(m wsOut) {
		wmu.Lock()
		defer wmu.Unlock()
		b, _ := json.Marshal(m)
		_ = conn.Write(ctx, websocket.MessageText, b)
	}

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return // client gone; deferred Kill stops the program
		}
		var in wsIn
		if err := json.Unmarshal(data, &in); err != nil {
			continue
		}
		switch in.Op {
		case "run":
			err := sess.Run(in.Toolchain, in.Files, in.Args,
				func(out string) { send(wsOut{Type: "out", Data: out}) },
				func(st string) { send(wsOut{Type: "status", Data: st}) },
				func(code int) { send(wsOut{Type: "exit", Code: code}) },
			)
			if err != nil {
				send(wsOut{Type: "error", Data: err.Error()})
			}
		case "stdin":
			sess.Stdin(in.Data)
		case "kill":
			sess.Kill()
		}
	}
}
