package ledgerreadwriterace

import (
	"fmt"
	"sync"
	"testing"

	"oral-history-release-desk/internal/policy"
	"oral-history-release-desk/internal/release"
	"oral-history-release-desk/internal/store"
)

func TestConcurrentLedgerReadersAreSynchronizedWithCommits(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := release.NewService(repository, policy.New())
	_, err = service.CreateCase(release.CreateCaseInput{
		ID: "case-base", Title: "并发读取基准案件", InterviewDate: "2025-02-03",
		IntendedUse: "研究", ConsentScope: []string{"研究"}, Actor: "整理员", IdempotencyKey: "create-base",
	})
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	errorsSeen := make(chan error, 64)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		<-start
		for i := 0; i < 32; i++ {
			id := fmt.Sprintf("case-write-%02d", i)
			_, createErr := service.CreateCase(release.CreateCaseInput{
				ID: id, Title: "并发写入案件", InterviewDate: "2025-02-03",
				IntendedUse: "研究", ConsentScope: []string{"研究"}, Actor: "整理员",
				IdempotencyKey: "create-" + id,
			})
			if createErr != nil {
				errorsSeen <- createErr
				return
			}
		}
	}()
	go func() {
		defer workers.Done()
		<-start
		for i := 0; i < 32; i++ {
			if _, getErr := service.Get("case-base"); getErr != nil {
				errorsSeen <- getErr
				return
			}
			_ = service.List()
			_ = repository.LedgerSequence()
		}
	}()
	close(start)
	workers.Wait()
	close(errorsSeen)
	for workerErr := range errorsSeen {
		t.Fatalf("并发流程出现非竞态错误: %v", workerErr)
	}
}
