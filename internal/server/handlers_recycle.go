package server

import (
	"encoding/json"
	"net/http"
)

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
	s.mutateJSON(w, r, &req, func() error {
		return s.wb.ConfirmTrash(req.PlaceholderIDs)
	})
}

type restoreReq struct {
	EntryID string `json:"entryId"`
}

func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	var req restoreReq
	s.mutateJSON(w, r, &req, func() error {
		return s.wb.Restore(req.EntryID)
	})
}

func (s *Server) handleEmptyRecycle(w http.ResponseWriter, r *http.Request) {
	s.lockState(w, func() error {
		return s.wb.EmptyRecycleBin()
	})
}

type moveRecycleDesktopReq struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

func (s *Server) handleMoveRecycleDesktop(w http.ResponseWriter, r *http.Request) {
	var req moveRecycleDesktopReq
	s.mutateJSON(w, r, &req, func() error {
		return s.wb.MoveRecycleToDesktop(req.Row, req.Col)
	})
}

type moveRecycleBoxReq struct {
	BoxID         string `json:"boxId"`
	CompartmentID string `json:"compartmentId"`
}

func (s *Server) handleMoveRecycleBox(w http.ResponseWriter, r *http.Request) {
	var req moveRecycleBoxReq
	s.mutateJSON(w, r, &req, func() error {
		return s.wb.MoveRecycleToBox(req.BoxID, req.CompartmentID)
	})
}
