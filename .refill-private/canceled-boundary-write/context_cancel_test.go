package canceledboundarywrite_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"oral-history-release-desk/internal/casefile"
	"oral-history-release-desk/internal/policy"
	"oral-history-release-desk/internal/release"
	"oral-history-release-desk/internal/store"
	"oral-history-release-desk/internal/web"
)

type stagedCancelContext struct {
	done    chan struct{}
	checked chan struct{}
	once    sync.Once
}

func newStagedCancelContext() *stagedCancelContext {
	return &stagedCancelContext{done: make(chan struct{}), checked: make(chan struct{})}
}

func (c *stagedCancelContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *stagedCancelContext) Done() <-chan struct{}       { return c.done }
func (c *stagedCancelContext) Value(any) any               { return nil }

func (c *stagedCancelContext) Err() error {
	select {
	case <-c.done:
		return context.Canceled
	default:
		c.once.Do(func() { close(c.checked) })
		return nil
	}
}

func TestCanceledBoundaryRequestDoesNotCommitAfterWriteQueue(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := release.NewService(repository, policy.New())
	createCase := func(id, key string) {
		t.Helper()
		_, err := service.CreateCase(release.CreateCaseInput{
			ID: id, Title: "原始边界", InterviewDate: "2025-01-01", IntendedUse: "研究",
			ConsentScope: []string{"研究"}, Actor: "整理员", IdempotencyKey: key,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	createCase("case-blocker", "create-blocker")
	createCase("case-canceled", "create-canceled")

	entered := make(chan struct{})
	releaseWriter := make(chan struct{})
	blockerResult := make(chan error, 1)
	go func() {
		_, _, executeErr := repository.Execute("case-blocker", 1, "case.boundary.revise", "block-writer", time.Now().UTC(), func(c *casefile.Case) error {
			close(entered)
			<-releaseWriter
			return c.ReviseBoundary(casefile.BoundaryInput{
				Title: "占用写协调器", InterviewDate: "2025-01-01", IntendedUse: "研究", ConsentScope: []string{"研究"},
			}, "整理员", time.Now().UTC())
		})
		blockerResult <- executeErr
	}()
	<-entered

	ctx := newStagedCancelContext()
	body := `{"expectedVersion":1,"idempotencyKey":"canceled-revise","actor":"整理员","title":"不应保存的边界","interviewDate":"2025-01-01","intendedUse":"研究","consentScope":["研究"],"restrictionTerms":[]}`
	request := httptest.NewRequest(http.MethodPut, "/api/v1/cases/case-canceled/boundary", strings.NewReader(body)).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	requestFinished := make(chan struct{})
	go func() {
		web.New(service).Handler().ServeHTTP(response, request)
		close(requestFinished)
	}()

	<-ctx.checked
	close(ctx.done)
	close(releaseWriter)
	if err := <-blockerResult; err != nil {
		t.Fatalf("占位写入失败: %v", err)
	}
	<-requestFinished

	stored, err := service.Get("case-canceled")
	if err != nil {
		t.Fatal(err)
	}
	if response.Code == http.StatusOK || stored.Version != 1 || stored.Title != "原始边界" {
		t.Fatalf("已取消请求仍被提交: status=%d version=%d title=%q", response.Code, stored.Version, stored.Title)
	}
}
