package closedstore_test

import (
	"strings"
	"testing"

	"oral-history-release-desk/internal/policy"
	"oral-history-release-desk/internal/release"
	"oral-history-release-desk/internal/store"
)

func TestClosedStoreRejectsCommandsWithoutPanic(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("打开测试账本: %v", err)
	}
	service := release.NewService(repository, policy.New())
	if err := repository.Close(); err != nil {
		t.Fatalf("关闭测试账本: %v", err)
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("关闭后的命令必须返回错误而不能 panic: %v", recovered)
		}
	}()
	_, err = service.CreateCase(release.CreateCaseInput{
		ID:               "case-after-close",
		Title:            "关闭后写入",
		InterviewDate:    "2024-06-01",
		IntendedUse:      "研究阅览",
		ConsentScope:     []string{"research"},
		RestrictionTerms: []string{"不得商业使用"},
		Actor:            "整理员",
		IdempotencyKey:   "create-after-close",
	})
	if err == nil {
		t.Fatal("关闭后的命令应返回资源已关闭错误")
	}
	if !strings.Contains(err.Error(), "关闭") {
		t.Fatalf("关闭后的错误应保留资源生命周期语义，得到: %v", err)
	}
}
