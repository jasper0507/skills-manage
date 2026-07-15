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
	s.mutateJSON(w, r, &req, func() error {
		return s.wb.SetClipboard(req.Mode, req.PlaceholderIDs)
	})
}

type pasteDesktopReq struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

func (s *Server) handlePasteDesktop(w http.ResponseWriter, r *http.Request) {
	var req pasteDesktopReq
	s.mutateJSON(w, r, &req, func() error {
		return s.wb.PasteToDesktop(req.Row, req.Col)
	})
}

type pasteBoxReq struct {
	BoxID         string `json:"boxId"`
	CompartmentID string `json:"compartmentId"`
}

func (s *Server) handlePasteBox(w http.ResponseWriter, r *http.Request) {
	var req pasteBoxReq
	s.mutateJSON(w, r, &req, func() error {
		return s.wb.PasteToBox(req.BoxID, req.CompartmentID)
	})
}

type phIDReq struct {
	PlaceholderID string `json:"placeholderId"`
}

func (s *Server) handleEnableMultiSelect(w http.ResponseWriter, r *http.Request) {
	var req phIDReq
	s.mutateJSON(w, r, &req, func() error {
		return s.wb.EnableMultiSelect(req.PlaceholderID)
	})
}

func (s *Server) handleDisableMultiSelect(w http.ResponseWriter, r *http.Request) {
	s.lockState(w, func() error {
		s.wb.DisableMultiSelect()
		return nil
	})
}

func (s *Server) handleToggleSelected(w http.ResponseWriter, r *http.Request) {
	var req phIDReq
	s.mutateJSON(w, r, &req, func() error {
		return s.wb.ToggleSelected(req.PlaceholderID)
	})
}
