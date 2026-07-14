// Package server is the thin localhost HTTP transport over Workbench.
// Domain rules live in Workbench; handlers only decode requests and encode views.
package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

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

// Handler returns the root HTTP handler (API + optional static files).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/state", s.handleState)
	mux.HandleFunc("POST /api/rescan", s.handleRescan)
	mux.HandleFunc("POST /api/placeholders/move-desktop", s.handleMovePlaceholderDesktop)
	mux.HandleFunc("POST /api/placeholders/move-many-desktop", s.handleMovePlaceholdersDesktop)
	mux.HandleFunc("POST /api/placeholders/move-box", s.handleMovePlaceholderBox)
	mux.HandleFunc("POST /api/placeholders/move-many-box", s.handleMovePlaceholdersBox)
	mux.HandleFunc("POST /api/boxes/compose", s.handleComposeBoxes)
	mux.HandleFunc("POST /api/boxes/move", s.handleMoveBox)
	mux.HandleFunc("POST /api/boxes/set-active", s.handleSetActiveCompartment)
	mux.HandleFunc("POST /api/boxes/eject", s.handleEjectCompartment)
	mux.HandleFunc("POST /api/boxes/rename-tag", s.handleRenameBoxTag)
	mux.HandleFunc("POST /api/boxes/rename-title", s.handleRenameBoxTitle)
	mux.HandleFunc("POST /api/boxes/delete", s.handleDeleteBox)
	mux.HandleFunc("POST /api/boxes/create-simple", s.handleCreateSimpleBox)
	mux.HandleFunc("POST /api/boxes/create-composite", s.handleCreateCompositeBox)
	mux.HandleFunc("POST /api/clipboard/set", s.handleSetClipboard)
	mux.HandleFunc("POST /api/clipboard/paste-desktop", s.handlePasteDesktop)
	mux.HandleFunc("POST /api/clipboard/paste-box", s.handlePasteBox)
	mux.HandleFunc("POST /api/multiselect/enable", s.handleEnableMultiSelect)
	mux.HandleFunc("POST /api/multiselect/disable", s.handleDisableMultiSelect)
	mux.HandleFunc("POST /api/multiselect/toggle", s.handleToggleSelected)
	mux.HandleFunc("POST /api/trash/plan", s.handlePlanTrash)
	mux.HandleFunc("POST /api/trash/confirm", s.handleConfirmTrash)
	mux.HandleFunc("POST /api/recycle/restore", s.handleRestore)
	mux.HandleFunc("POST /api/recycle/empty", s.handleEmptyRecycle)

	if s.static != nil {
		fileServer := http.FileServer(http.FS(s.static))
		mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Serve index.html for /
			if r.URL.Path == "/" {
				data, err := fs.ReadFile(s.static, "index.html")
				if err != nil {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write(data)
				return
			}
			fileServer.ServeHTTP(w, r)
		}))
	}

	return mux
}

// --- response types (JSON view of Workbench; no second domain model) ---

type stateResponse struct {
	Desk       workbench.Desk           `json:"desk"`
	RecycleBin []workbench.RecycleView  `json:"recycleBin"`
}

type errorBody struct {
	Error string `json:"error"`
}

func (s *Server) writeState(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	resp := stateResponse{
		Desk:       s.wb.Desk(),
		RecycleBin: s.wb.RecycleBin(),
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

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writeState(w)
}

func (s *Server) handleRescan(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.wb.Rescan(); err != nil {
		s.writeErr(w, http.StatusInternalServerError, err)
		return
	}
	s.writeState(w)
}

type moveDesktopReq struct {
	PlaceholderID string `json:"placeholderId"`
	Row           int    `json:"row"`
	Col           int    `json:"col"`
}

func (s *Server) handleMovePlaceholderDesktop(w http.ResponseWriter, r *http.Request) {
	var req moveDesktopReq
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.wb.MovePlaceholderToDesktop(req.PlaceholderID, req.Row, req.Col); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeState(w)
}

type moveManyDesktopReq struct {
	PlaceholderIDs []string `json:"placeholderIds"`
	Row            int      `json:"row"`
	Col            int      `json:"col"`
}

func (s *Server) handleMovePlaceholdersDesktop(w http.ResponseWriter, r *http.Request) {
	var req moveManyDesktopReq
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.wb.MovePlaceholdersToDesktop(req.PlaceholderIDs, req.Row, req.Col); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeState(w)
}

type moveBoxReq struct {
	PlaceholderID string `json:"placeholderId"`
	BoxID         string `json:"boxId"`
	CompartmentID string `json:"compartmentId"`
}

func (s *Server) handleMovePlaceholderBox(w http.ResponseWriter, r *http.Request) {
	var req moveBoxReq
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.wb.MovePlaceholderToBox(req.PlaceholderID, req.BoxID, req.CompartmentID); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeState(w)
}

type moveManyBoxReq struct {
	PlaceholderIDs []string `json:"placeholderIds"`
	BoxID          string   `json:"boxId"`
	CompartmentID  string   `json:"compartmentId"`
}

func (s *Server) handleMovePlaceholdersBox(w http.ResponseWriter, r *http.Request) {
	var req moveManyBoxReq
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.wb.MovePlaceholdersToBox(req.PlaceholderIDs, req.BoxID, req.CompartmentID); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeState(w)
}

type composeReq struct {
	SourceBoxID string `json:"sourceBoxId"`
	TargetBoxID string `json:"targetBoxId"`
}

func (s *Server) handleComposeBoxes(w http.ResponseWriter, r *http.Request) {
	var req composeReq
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.wb.ComposeBoxes(req.SourceBoxID, req.TargetBoxID); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeState(w)
}

type moveBoxPosReq struct {
	BoxID string  `json:"boxId"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
}

func (s *Server) handleMoveBox(w http.ResponseWriter, r *http.Request) {
	var req moveBoxPosReq
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.wb.MoveBox(req.BoxID, req.X, req.Y); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeState(w)
}

type setActiveReq struct {
	BoxID         string `json:"boxId"`
	CompartmentID string `json:"compartmentId"`
}

func (s *Server) handleSetActiveCompartment(w http.ResponseWriter, r *http.Request) {
	var req setActiveReq
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.wb.SetActiveCompartment(req.BoxID, req.CompartmentID); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeState(w)
}

type ejectReq struct {
	BoxID         string  `json:"boxId"`
	CompartmentID string  `json:"compartmentId"`
	X             float64 `json:"x"`
	Y             float64 `json:"y"`
}

func (s *Server) handleEjectCompartment(w http.ResponseWriter, r *http.Request) {
	var req ejectReq
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.wb.EjectCompartment(req.BoxID, req.CompartmentID, req.X, req.Y); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeState(w)
}

type renameTagReq struct {
	BoxID         string `json:"boxId"`
	Tag           string `json:"tag"`
	CompartmentID string `json:"compartmentId"`
}

func (s *Server) handleRenameBoxTag(w http.ResponseWriter, r *http.Request) {
	var req renameTagReq
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.wb.RenameBoxTag(req.BoxID, req.Tag, req.CompartmentID); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeState(w)
}

type renameTitleReq struct {
	BoxID string `json:"boxId"`
	Title string `json:"title"`
}

func (s *Server) handleRenameBoxTitle(w http.ResponseWriter, r *http.Request) {
	var req renameTitleReq
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.wb.RenameBoxTitle(req.BoxID, req.Title); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeState(w)
}

type idReq struct {
	BoxID string `json:"boxId"`
}

func (s *Server) handleDeleteBox(w http.ResponseWriter, r *http.Request) {
	var req idReq
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.wb.DeleteBox(req.BoxID); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeState(w)
}

type createSimpleReq struct {
	Tag string  `json:"tag"`
	X   float64 `json:"x"`
	Y   float64 `json:"y"`
}

func (s *Server) handleCreateSimpleBox(w http.ResponseWriter, r *http.Request) {
	var req createSimpleReq
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.wb.CreateSimpleBox(req.Tag, req.X, req.Y); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeState(w)
}

type createCompositeReq struct {
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
	X     float64  `json:"x"`
	Y     float64  `json:"y"`
}

func (s *Server) handleCreateCompositeBox(w http.ResponseWriter, r *http.Request) {
	var req createCompositeReq
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.wb.CreateCompositeBox(req.Title, req.Tags, req.X, req.Y); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeState(w)
}

type clipboardReq struct {
	Mode           string   `json:"mode"`
	PlaceholderIDs []string `json:"placeholderIds"`
}

func (s *Server) handleSetClipboard(w http.ResponseWriter, r *http.Request) {
	var req clipboardReq
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.wb.SetClipboard(req.Mode, req.PlaceholderIDs); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeState(w)
}

type pasteDesktopReq struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

func (s *Server) handlePasteDesktop(w http.ResponseWriter, r *http.Request) {
	var req pasteDesktopReq
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.wb.PasteToDesktop(req.Row, req.Col); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeState(w)
}

type pasteBoxReq struct {
	BoxID         string `json:"boxId"`
	CompartmentID string `json:"compartmentId"`
}

func (s *Server) handlePasteBox(w http.ResponseWriter, r *http.Request) {
	var req pasteBoxReq
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.wb.PasteToBox(req.BoxID, req.CompartmentID); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeState(w)
}

type phIDReq struct {
	PlaceholderID string `json:"placeholderId"`
}

func (s *Server) handleEnableMultiSelect(w http.ResponseWriter, r *http.Request) {
	var req phIDReq
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.wb.EnableMultiSelect(req.PlaceholderID); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeState(w)
}

func (s *Server) handleDisableMultiSelect(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wb.DisableMultiSelect()
	s.writeState(w)
}

func (s *Server) handleToggleSelected(w http.ResponseWriter, r *http.Request) {
	var req phIDReq
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.wb.ToggleSelected(req.PlaceholderID); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeState(w)
}

type trashIDsReq struct {
	PlaceholderIDs []string `json:"placeholderIds"`
}

func (s *Server) handlePlanTrash(w http.ResponseWriter, r *http.Request) {
	var req trashIDsReq
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	plan, err := s.wb.PlanTrash(req.PlaceholderIDs)
	if err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(plan)
}

func (s *Server) handleConfirmTrash(w http.ResponseWriter, r *http.Request) {
	var req trashIDsReq
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.wb.ConfirmTrash(req.PlaceholderIDs); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeState(w)
}

type restoreReq struct {
	EntryID string `json:"entryId"`
}

func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	var req restoreReq
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.wb.Restore(req.EntryID); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeState(w)
}

func (s *Server) handleEmptyRecycle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.wb.EmptyRecycleBin(); err != nil {
		s.writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.writeState(w)
}

// ListenAndServe binds addr (e.g. "127.0.0.1:0" for ephemeral) and serves until
// the process is stopped. Returns the bound URL (http://host:port) once listening.
func ListenAndServe(addr string, handler http.Handler) (url string, serve func() error, closeFn func() error, err error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", nil, nil, err
	}
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	url = "http://" + ln.Addr().String()
	// Prefer 127.0.0.1 over [::] for browser friendliness when dual-stack.
	if strings.HasPrefix(ln.Addr().String(), "[::]:") {
		url = "http://127.0.0.1" + strings.TrimPrefix(ln.Addr().String(), "[::]")
	}
	serve = func() error {
		err := srv.Serve(ln)
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
	closeFn = func() error {
		return srv.Close()
	}
	return url, serve, closeFn, nil
}
