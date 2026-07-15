package server

import (
	"io/fs"
	"net/http"
)

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
	mux.HandleFunc("POST /api/recycle/move-desktop", s.handleMoveRecycleDesktop)
	mux.HandleFunc("POST /api/recycle/move-box", s.handleMoveRecycleBox)

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
