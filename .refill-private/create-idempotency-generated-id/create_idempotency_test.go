package createidempotency_test

import (
	"testing"

	"oral-history-release-desk/internal/policy"
	"oral-history-release-desk/internal/release"
	"oral-history-release-desk/internal/store"
)

func TestCreateWithoutIDReplaysSameCase(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := release.NewService(repository, policy.New())
	input := release.CreateCaseInput{
		Title: "自动编号幂等测试", InterviewDate: "2026-01-02", IntendedUse: "研究",
		ConsentScope: []string{"研究"}, Actor: "整理员", IdempotencyKey: "same-create-request",
	}
	first, err := service.CreateCase(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateCase(input)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Idempotent || second.Case.ID != first.Case.ID || len(service.List()) != 1 {
		t.Fatalf("TestCreateWithoutIDReplaysSameCase: 相同建案请求被创建为多个案件: first=%s second=%s replayed=%v count=%d",
			first.Case.ID, second.Case.ID, second.Idempotent, len(service.List()))
	}
}
