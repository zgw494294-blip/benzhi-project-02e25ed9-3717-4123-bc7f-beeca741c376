package commandlockregistryrace_test

import (
	"fmt"
	"sync"
	"testing"

	"oral-history-release-desk/internal/policy"
	"oral-history-release-desk/internal/release"
	"oral-history-release-desk/internal/store"
)

func TestConcurrentCommandLockRegistryIsSynchronized(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("打开测试账本: %v", err)
	}
	t.Cleanup(func() { _ = repository.Close() })
	service := release.NewService(repository, &policy.Engine{})

	const workers = 24
	start := make(chan struct{})
	var ready sync.WaitGroup
	var done sync.WaitGroup
	ready.Add(workers)
	done.Add(workers)
	errorsByWorker := make([]error, workers)
	for worker := 0; worker < workers; worker++ {
		worker := worker
		go func() {
			defer done.Done()
			ready.Done()
			<-start
			caseID := fmt.Sprintf("concurrent-case-%02d", worker)
			_, errorsByWorker[worker] = service.CreateCase(release.CreateCaseInput{
				ID: caseID, Title: "并发案件", InterviewDate: "2024-03-01",
				IntendedUse: "研究阅览", ConsentScope: []string{"公开展示"},
				Actor: "tester", IdempotencyKey: "create-" + caseID,
			})
		}()
	}
	ready.Wait()
	close(start)
	done.Wait()
	for worker, err := range errorsByWorker {
		if err != nil {
			t.Fatalf("第 %d 个并发命令失败: %v", worker, err)
		}
	}
}
