package server

import (
	"net/http"
)

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
