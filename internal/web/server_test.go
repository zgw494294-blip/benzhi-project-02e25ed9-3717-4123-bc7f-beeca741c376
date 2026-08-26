package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oral-history-release-desk/internal/policy"
	"oral-history-release-desk/internal/release"
	"oral-history-release-desk/internal/store"
)

func TestWorkbenchAndJSONValidation(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := New(release.NewService(repository, policy.New())).Handler()
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "<!doctype html>") || !strings.Contains(page.Body.String(), "口述史开放治理台") {
		t.Fatalf("工作台响应无效: %d", page.Code)
	}
	bad := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/cases", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "text/plain")
	handler.ServeHTTP(bad, request)
	if bad.Code != http.StatusBadRequest || !strings.Contains(bad.Body.String(), "bad_request") {
		t.Fatalf("内容类型错误映射无效: %d %s", bad.Code, bad.Body.String())
	}
}
