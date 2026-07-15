package server

import (
	"net/http"
)

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
