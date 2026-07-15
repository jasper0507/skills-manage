package server

import (
	"net/http"
)

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
