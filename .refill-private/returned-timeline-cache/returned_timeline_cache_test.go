package returned_timeline_cache_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"oral-history-release-desk/internal/casefile"
	"oral-history-release-desk/internal/policy"
	"oral-history-release-desk/internal/release"
	"oral-history-release-desk/internal/store"
	"oral-history-release-desk/internal/web"
)

func TestReturnedCaseTimelineRefreshesAfterTargetedRevision(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := release.NewService(repository, policy.New())
	returned := prepareReturnedCase(t, service)
	handler := web.New(service).Handler()

	before := fetchTimeline(t, handler, returned.ID)
	if before.Version != returned.Version || len(before.Timeline) != int(returned.Version) {
		t.Fatalf("未能在退回状态建立时间线快照: version=%d events=%d", before.Version, len(before.Timeline))
	}

	revision := release.ReviseSegmentInput{
		CommandMeta: release.CommandMeta{ExpectedVersion: returned.Version, IdempotencyKey: "revise-returned", Actor: "整理员"},
		ID:          "s1", Sequence: 1, StartMillis: 0, EndMillis: 3000,
		OriginalText: "受访者曾居住在老城区", SpeakerLabel: "受访者",
	}
	body, err := json.Marshal(revision)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPut, "/api/v1/cases/returned-case/segments/s1", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("定向整改请求失败: status=%d body=%s", response.Code, response.Body.String())
	}

	after := fetchTimeline(t, handler, returned.ID)
	if after.Version != returned.Version+1 {
		t.Fatalf("定向整改提交后时间线仍为缓存版本: got=%d want=%d", after.Version, returned.Version+1)
	}
	if len(after.Timeline) != len(before.Timeline)+1 || after.Timeline[len(after.Timeline)-1].Type != "segment.revised" {
		t.Fatalf("定向整改事件未出现在时间线: events=%d last=%q", len(after.Timeline), after.Timeline[len(after.Timeline)-1].Type)
	}
}

func prepareReturnedCase(t *testing.T, service *release.Service) *casefile.Case {
	t.Helper()
	created, err := service.CreateCase(release.CreateCaseInput{
		ID: "returned-case", Title: "退回整改时间线", InterviewDate: "2025-03-01",
		IntendedUse: "公开研究", ConsentScope: []string{"公开研究"}, RestrictionTerms: []string{"不得披露姓名"},
		Actor: "整理员", IdempotencyKey: "create",
	})
	if err != nil {
		t.Fatal(err)
	}
	c := created.Case
	added, err := service.AddSegment(c.ID, release.AddSegmentInput{
		CommandMeta: meta(c, "segment", "整理员"), ID: "s1", Sequence: 1, StartMillis: 0, EndMillis: 3000,
		OriginalText: "张某曾居住在北街", SpeakerLabel: "张某", SensitivityTags: []string{"姓名"},
	})
	if err != nil {
		t.Fatal(err)
	}
	c = added.Case
	command, commandErr := service.Freeze(c.ID, meta(c, "freeze", "整理员"))
	c = mustCommand(t, command, commandErr)
	command, commandErr = service.RunCheck(c.ID, meta(c, "check", "整理员"))
	c = mustCommand(t, command, commandErr)
	for i, finding := range c.OpenBlocks() {
		decided, decideErr := service.DecideFinding(c.ID, release.DecideFindingInput{
			CommandMeta: meta(c, fmt.Sprintf("decision-%d", i), "整理员"), FindingID: finding.ID,
			Action: casefile.ActionRedact, Rationale: "保护受访者身份",
		})
		if decideErr != nil {
			t.Fatal(decideErr)
		}
		c = decided.Case
	}
	command, commandErr = service.GeneratePreview(c.ID, meta(c, "preview", "整理员"))
	c = mustCommand(t, command, commandErr)
	command, commandErr = service.SubmitReview(c.ID, meta(c, "submit", "整理员"))
	c = mustCommand(t, command, commandErr)
	returned, err := service.ReviewDecision(c.ID, release.ReviewDecisionInput{
		CommandMeta: meta(c, "return", "复核员"), Outcome: casefile.ReviewReturned,
		ReasonCode: "IDENTITY", Reason: "需要改写身份线索", AffectedSegmentIDs: []string{"s1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return returned.Case
}

func fetchTimeline(t *testing.T, handler http.Handler, caseID string) release.TimelineResult {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/cases/"+caseID+"/timeline", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("读取时间线失败: status=%d body=%s", response.Code, response.Body.String())
	}
	var result release.TimelineResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func meta(c *casefile.Case, key, actor string) release.CommandMeta {
	return release.CommandMeta{ExpectedVersion: c.Version, IdempotencyKey: key, Actor: actor}
}

func mustCommand(t *testing.T, result release.CommandResult, err error) *casefile.Case {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	return result.Case
}
