// Package server is the thin localhost HTTP transport over Workbench.
// Domain rules live in Workbench; handlers only decode requests and encode views.
//
// File layout:
//
//	server.go, router.go, listen.go, run.go — core
//	handlers_*.go — thin JSON handlers grouped by concern
package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"sync"

	"github.com/jasper0507/skills-manage/internal/workbench"
)

// Server exposes Workbench over HTTP for the embedded 分类工作台 UI.
type Server struct {
	wb     *workbench.Workbench
	mu     sync.Mutex
	static fs.FS // optional embedded UI; nil → API only
}

// New wraps an already-opened Workbench. Static may be nil (tests / API-only).
func New(wb *workbench.Workbench) *Server {
	return &Server{wb: wb}
}

// WithStatic attaches embedded UI assets (index.html, app.js, styles.css, …).
func (s *Server) WithStatic(static fs.FS) *Server {
	s.static = static
	return s
}

// --- response types (JSON view of Workbench; no second domain model) ---

type stateResponse struct {
	Desk       workbench.Desk          `json:"desk"`
	RecycleBin []workbench.RecycleView `json:"recycleBin"`
}

type errorBody struct {
	Error string `json:"error"`
}

func (s *Server) writeState(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	bin := s.wb.RecycleBin()
	if bin == nil {
		bin = []workbench.RecycleView{}
	}
	resp := stateResponse{
		Desk:       s.wb.Desk(),
		RecycleBin: bin,
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) writeErr(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorBody{Error: err.Error()})
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	return nil
}

// lockState runs fn under the server mutex and writes desk state on success.
// Domain errors map to 400 (thin UI contract).
func (s *Server) lockState(w http.ResponseWriter, fn func() error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := fn(); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeState(w)
}

// mutateJSON decodes JSON body into dst, then lockState(fn).
func (s *Server) mutateJSON(w http.ResponseWriter, r *http.Request, dst any, fn func() error) {
	if err := decodeJSON(r, dst); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.lockState(w, fn)
}
