package server

import (
	"net/http"
)

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
