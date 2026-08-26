package web

import (
	"net/http"
	"strconv"
	"strings"

	"oral-history-release-desk/internal/casefile"
	"oral-history-release-desk/internal/release"
)

func commandMeta(w http.ResponseWriter, r *http.Request) (release.CommandMeta, error) {
	var input release.CommandMeta
	err := decodeJSON(w, r, &input)
	return input, err
}

func (s *Server) FreezeCaseHandler(w http.ResponseWriter, r *http.Request) {
	s.handleMetaCommand(w, r, s.service.Freeze)
}

func (s *Server) RunCheckHandler(w http.ResponseWriter, r *http.Request) {
	s.handleMetaCommand(w, r, s.service.RunCheck)
}

func (s *Server) RunTargetedCheckHandler(w http.ResponseWriter, r *http.Request) {
	s.handleMetaCommand(w, r, s.service.RunTargetedCheck)
}

func (s *Server) GeneratePreviewHandler(w http.ResponseWriter, r *http.Request) {
	s.handleMetaCommand(w, r, s.service.GeneratePreview)
}

func (s *Server) SubmitReviewHandler(w http.ResponseWriter, r *http.Request) {
	s.handleMetaCommand(w, r, s.service.SubmitReview)
}

func (s *Server) handleMetaCommand(w http.ResponseWriter, r *http.Request, command func(string, release.CommandMeta) (release.CommandResult, error)) {
	id, err := caseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	meta, err := commandMeta(w, r)
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := command(id, meta)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) DecideFindingHandler(w http.ResponseWriter, r *http.Request) {
	id, err := caseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var input release.DecideFindingInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, err)
		return
	}
	result, err := s.service.DecideFinding(id, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) DecideFindingsHandler(w http.ResponseWriter, r *http.Request) {
	id, err := caseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var input release.DecideFindingsInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, err)
		return
	}
	result, err := s.service.DecideFindings(id, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) RiskSummaryHandler(w http.ResponseWriter, r *http.Request) {
	id, err := caseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	query := r.URL.Query()
	filter := release.RiskSummaryFilter{
		Severity: query.Get("severity"), RuleCode: query.Get("ruleCode"), SegmentID: query.Get("segmentId"),
	}
	if raw, present := query["changed"]; present {
		if len(raw) != 1 || (raw[0] != "true" && raw[0] != "false") {
			writeError(w, casefile.ValidationError{Field: "changed", Message: "changed 必须为 true 或 false"})
			return
		}
		value, _ := strconv.ParseBool(raw[0])
		filter.Changed = &value
	}
	result, err := s.service.RiskSummary(id, filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) ReviewDecisionHandler(w http.ResponseWriter, r *http.Request) {
	id, err := caseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var input release.ReviewDecisionInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, err)
		return
	}
	result, err := s.service.ReviewDecision(id, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) FinalApprovalHandler(w http.ResponseWriter, r *http.Request) {
	id, err := caseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	meta, err := commandMeta(w, r)
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := s.service.FinalApprove(id, meta)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) TimelineHandler(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, map[string]any{"caseId": id, "version": c.Version, "timeline": c.Timeline})
}

func (s *Server) ReleasePackageHandler(w http.ResponseWriter, r *http.Request) {
	id, err := caseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	outcome := casefile.ReviewOutcome(strings.TrimSpace(r.URL.Query().Get("outcome")))
	result, err := s.service.EvidenceCatalog(id, release.EvidenceFilter{RuleCode: r.URL.Query().Get("ruleCode"), Outcome: outcome})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) DownloadPackageHandler(w http.ResponseWriter, r *http.Request) {
	id, err := caseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	filename, data, err := s.service.PackageDownload(id)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) VerifyPackageHandler(w http.ResponseWriter, r *http.Request) {
	id, err := caseID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := s.service.VerifyPackage(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
