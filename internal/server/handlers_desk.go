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
	s.mutateJSON(w, r, &req, func() error {
		return s.wb.MovePlaceholderToDesktop(req.PlaceholderID, req.Row, req.Col)
	})
}

type moveManyDesktopReq struct {
	PlaceholderIDs []string `json:"placeholderIds"`
	Row            int      `json:"row"`
	Col            int      `json:"col"`
}

func (s *Server) handleMovePlaceholdersDesktop(w http.ResponseWriter, r *http.Request) {
	var req moveManyDesktopReq
	s.mutateJSON(w, r, &req, func() error {
		return s.wb.MovePlaceholdersToDesktop(req.PlaceholderIDs, req.Row, req.Col)
	})
}

type moveBoxReq struct {
	PlaceholderID string `json:"placeholderId"`
	BoxID         string `json:"boxId"`
	CompartmentID string `json:"compartmentId"`
}

func (s *Server) handleMovePlaceholderBox(w http.ResponseWriter, r *http.Request) {
	var req moveBoxReq
	s.mutateJSON(w, r, &req, func() error {
		return s.wb.MovePlaceholderToBox(req.PlaceholderID, req.BoxID, req.CompartmentID)
	})
}

type moveManyBoxReq struct {
	PlaceholderIDs []string `json:"placeholderIds"`
	BoxID          string   `json:"boxId"`
	CompartmentID  string   `json:"compartmentId"`
}

func (s *Server) handleMovePlaceholdersBox(w http.ResponseWriter, r *http.Request) {
	var req moveManyBoxReq
	s.mutateJSON(w, r, &req, func() error {
		return s.wb.MovePlaceholdersToBox(req.PlaceholderIDs, req.BoxID, req.CompartmentID)
	})
}
