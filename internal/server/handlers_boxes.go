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
	s.mutateJSON(w, r, &req, func() error {
		return s.wb.ComposeBoxes(req.SourceBoxID, req.TargetBoxID)
	})
}

type moveBoxPosReq struct {
	BoxID string  `json:"boxId"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
}

func (s *Server) handleMoveBox(w http.ResponseWriter, r *http.Request) {
	var req moveBoxPosReq
	s.mutateJSON(w, r, &req, func() error {
		return s.wb.MoveBox(req.BoxID, req.X, req.Y)
	})
}

type setActiveReq struct {
	BoxID         string `json:"boxId"`
	CompartmentID string `json:"compartmentId"`
}

func (s *Server) handleSetActiveCompartment(w http.ResponseWriter, r *http.Request) {
	var req setActiveReq
	s.mutateJSON(w, r, &req, func() error {
		return s.wb.SetActiveCompartment(req.BoxID, req.CompartmentID)
	})
}

type ejectReq struct {
	BoxID         string  `json:"boxId"`
	CompartmentID string  `json:"compartmentId"`
	X             float64 `json:"x"`
	Y             float64 `json:"y"`
}

func (s *Server) handleEjectCompartment(w http.ResponseWriter, r *http.Request) {
	var req ejectReq
	s.mutateJSON(w, r, &req, func() error {
		return s.wb.EjectCompartment(req.BoxID, req.CompartmentID, req.X, req.Y)
	})
}

type renameTagReq struct {
	BoxID         string `json:"boxId"`
	Tag           string `json:"tag"`
	CompartmentID string `json:"compartmentId"`
}

func (s *Server) handleRenameBoxTag(w http.ResponseWriter, r *http.Request) {
	var req renameTagReq
	s.mutateJSON(w, r, &req, func() error {
		return s.wb.RenameBoxTag(req.BoxID, req.Tag, req.CompartmentID)
	})
}

type renameTitleReq struct {
	BoxID string `json:"boxId"`
	Title string `json:"title"`
}

func (s *Server) handleRenameBoxTitle(w http.ResponseWriter, r *http.Request) {
	var req renameTitleReq
	s.mutateJSON(w, r, &req, func() error {
		return s.wb.RenameBoxTitle(req.BoxID, req.Title)
	})
}

type idReq struct {
	BoxID string `json:"boxId"`
}

func (s *Server) handleDeleteBox(w http.ResponseWriter, r *http.Request) {
	var req idReq
	s.mutateJSON(w, r, &req, func() error {
		return s.wb.DeleteBox(req.BoxID)
	})
}

type createSimpleReq struct {
	Tag string  `json:"tag"`
	X   float64 `json:"x"`
	Y   float64 `json:"y"`
}

func (s *Server) handleCreateSimpleBox(w http.ResponseWriter, r *http.Request) {
	var req createSimpleReq
	s.mutateJSON(w, r, &req, func() error {
		_, err := s.wb.CreateSimpleBox(req.Tag, req.X, req.Y)
		return err
	})
}

type createCompositeReq struct {
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
	X     float64  `json:"x"`
	Y     float64  `json:"y"`
}

func (s *Server) handleCreateCompositeBox(w http.ResponseWriter, r *http.Request) {
	var req createCompositeReq
	s.mutateJSON(w, r, &req, func() error {
		_, err := s.wb.CreateCompositeBox(req.Title, req.Tags, req.X, req.Y)
		return err
	})
}
