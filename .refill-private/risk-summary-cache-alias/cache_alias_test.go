package risk_summary_cache_alias_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"oral-history-release-desk/internal/casefile"
	"oral-history-release-desk/internal/policy"
	"oral-history-release-desk/internal/release"
	"oral-history-release-desk/internal/store"
	"oral-history-release-desk/internal/web"
)

func TestFilteredRiskSummaryDoesNotPolluteCachedFullSummary(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := release.NewService(repository, policy.New())
	created, err := service.CreateCase(release.CreateCaseInput{
		ID: "case-risk-cache", Title: "风险缓存隔离", InterviewDate: "2025-04-08",
		IntendedUse: "公开研究", ConsentScope: []string{"公开研究"},
		Actor: "整理员", IdempotencyKey: "create",
	})
	if err != nil {
		t.Fatal(err)
	}
	c := created.Case
	segments := []release.AddSegmentInput{
		{ID: "segment-block", Sequence: 1, StartMillis: 0, EndMillis: 1000, OriginalText: "张某住在北街", SpeakerLabel: "张某", SensitivityTags: []string{"姓名"}},
		{ID: "segment-pass", Sequence: 2, StartMillis: 1000, EndMillis: 2000, OriginalText: "社区活动记录", SpeakerLabel: "受访者"},
		{ID: "segment-warning", Sequence: 3, StartMillis: 2000, EndMillis: 3000, OriginalText: "一次财务讨论", SpeakerLabel: "受访者", SensitivityTags: []string{"财务"}},
	}
	for i := range segments {
		segments[i].CommandMeta = commandMeta(c, "segment-"+segments[i].ID)
		result, addErr := service.AddSegment(c.ID, segments[i])
		if addErr != nil {
			t.Fatal(addErr)
		}
		c = result.Case
	}
	frozen, err := service.Freeze(c.ID, commandMeta(c, "freeze"))
	if err != nil {
		t.Fatal(err)
	}
	checked, err := service.RunCheck(c.ID, commandMeta(frozen.Case, "check"))
	if err != nil {
		t.Fatal(err)
	}

	handler := web.New(service).Handler()
	fullBefore := fetchSummary(t, handler, "/api/v1/cases/"+checked.Case.ID+"/risk-summary")
	if len(fullBefore.Findings) != 3 || severityCount(fullBefore, casefile.SeverityBlock) != 1 || severityCount(fullBefore, casefile.SeverityPass) != 1 {
		t.Fatalf("测试前提不成立: %#v", fullBefore.Findings)
	}
	passOnly := fetchSummary(t, handler, "/api/v1/cases/"+checked.Case.ID+"/risk-summary?severity=pass")
	if len(passOnly.Findings) != 1 || passOnly.Findings[0].Severity != casefile.SeverityPass {
		t.Fatalf("筛选响应不正确: %#v", passOnly.Findings)
	}
	fullAfter := fetchSummary(t, handler, "/api/v1/cases/"+checked.Case.ID+"/risk-summary")
	if severityCount(fullAfter, casefile.SeverityBlock) != 1 || fullAfter.Findings[0].FindingID != fullBefore.Findings[0].FindingID {
		t.Fatalf("全量风险摘要被筛选请求污染: before=%#v after=%#v", fullBefore.Findings, fullAfter.Findings)
	}
}

func commandMeta(c *casefile.Case, key string) release.CommandMeta {
	return release.CommandMeta{ExpectedVersion: c.Version, IdempotencyKey: key, Actor: "整理员"}
}

func fetchSummary(t *testing.T, handler http.Handler, target string) policy.RiskSummary {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("风险摘要响应状态错误: %d %s", response.Code, response.Body.String())
	}
	var result policy.RiskSummary
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func severityCount(summary policy.RiskSummary, severity casefile.Severity) int {
	count := 0
	for _, finding := range summary.Findings {
		if finding.Severity == severity {
			count++
		}
	}
	return count
}
