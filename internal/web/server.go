package web

import (
	"net/http"
	"time"

	"oral-history-release-desk/internal/release"
)

type Server struct {
	service *release.Service
	mux     *http.ServeMux
}

func New(service *release.Service) *Server {
	s := &Server{service: service, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return securityHeaders(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /", s.WorkbenchHandler)
	s.mux.HandleFunc("GET /assets/app.css", s.CSSHandler)
	s.mux.HandleFunc("GET /assets/app.js", s.JavaScriptHandler)
	s.mux.HandleFunc("GET /api/v1/health", s.HealthHandler)
	s.mux.HandleFunc("GET /api/v1/cases", s.ListCasesHandler)
	s.mux.HandleFunc("POST /api/v1/cases", s.CreateCaseHandler)
	s.mux.HandleFunc("GET /api/v1/cases/{caseID}", s.GetCaseHandler)
	s.mux.HandleFunc("PUT /api/v1/cases/{caseID}/boundary", s.ReviseBoundaryHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{caseID}/segments", s.AddSegmentHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{caseID}/segments/batch", s.AddSegmentsHandler)
	s.mux.HandleFunc("PUT /api/v1/cases/{caseID}/segments/{segmentID}", s.ReviseSegmentHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{caseID}/freeze", s.FreezeCaseHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{caseID}/checks", s.RunCheckHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{caseID}/rechecks", s.RunTargetedCheckHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{caseID}/decisions", s.DecideFindingHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{caseID}/decisions/batch", s.DecideFindingsHandler)
	s.mux.HandleFunc("GET /api/v1/cases/{caseID}/risk-summary", s.RiskSummaryHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{caseID}/preview", s.GeneratePreviewHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{caseID}/review/submit", s.SubmitReviewHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{caseID}/review/decision", s.ReviewDecisionHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{caseID}/approval", s.FinalApprovalHandler)
	s.mux.HandleFunc("GET /api/v1/cases/{caseID}/timeline", s.TimelineHandler)
	s.mux.HandleFunc("GET /api/v1/cases/{caseID}/package", s.ReleasePackageHandler)
	s.mux.HandleFunc("GET /api/v1/cases/{caseID}/package/download", s.DownloadPackageHandler)
	s.mux.HandleFunc("GET /api/v1/cases/{caseID}/package/verify", s.VerifyPackageHandler)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) HealthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok", "service": "oral-history-release-desk", "time": time.Now().UTC(),
	})
}
