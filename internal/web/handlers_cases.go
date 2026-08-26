package web

import (
	"net/http"
	"strings"

	"oral-history-release-desk/internal/release"
)

func (s *Server) ListCasesHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"cases": s.service.List()})
}

func (s *Server) CreateCaseHandler(w http.ResponseWriter, r *http.Request) {
	var input release.CreateCaseInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, err)
		return
	}
	result, err := s.service.CreateCase(input)
	if err != nil {
		writeError(w, err)
		return
	}
	status := http.StatusCreated
	if result.Idempotent {
		status = http.StatusOK
	}
	writeJSON(w, status, result)
}

func (s *Server) GetCaseHandler(w http.ResponseWriter, r *http.Request) {
	id, err := caseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	c, err := s.service.Get(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"case": c, "openBlocks": c.OpenBlocks()})
}

func (s *Server) ReviseBoundaryHandler(w http.ResponseWriter, r *http.Request) {
	id, err := caseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var input release.ReviseBoundaryInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, err)
		return
	}
	result, err := s.service.ReviseBoundary(id, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) AddSegmentHandler(w http.ResponseWriter, r *http.Request) {
	id, err := caseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var input release.AddSegmentInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, err)
		return
	}
	result, err := s.service.AddSegment(id, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) AddSegmentsHandler(w http.ResponseWriter, r *http.Request) {
	id, err := caseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var input release.AddSegmentsInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, err)
		return
	}
	result, err := s.service.AddSegments(id, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) ReviseSegmentHandler(w http.ResponseWriter, r *http.Request) {
	id, err := caseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var input release.ReviseSegmentInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, err)
		return
	}
	pathSegment := strings.TrimSpace(r.PathValue("segmentID"))
	if input.ID != "" && input.ID != pathSegment {
		writeError(w, releaseInputError("路径片段 ID 与请求体不一致"))
		return
	}
	input.ID = pathSegment
	result, err := s.service.ReviseSegment(id, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type releaseInputError string

func (e releaseInputError) Error() string { return string(e) }
