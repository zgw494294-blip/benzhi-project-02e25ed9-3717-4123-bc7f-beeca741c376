package package_download_cache_race_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"testing"
	"time"

	"oral-history-release-desk/internal/casefile"
	"oral-history-release-desk/internal/policy"
	"oral-history-release-desk/internal/release"
	"oral-history-release-desk/internal/store"
	"oral-history-release-desk/internal/web"
)

func TestConcurrentPackageDownloadsShareCacheSafely(t *testing.T) {
	previousProcs := runtime.GOMAXPROCS(4)
	defer runtime.GOMAXPROCS(previousProcs)

	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	caseID := "case-concurrent-download"
	sealedAt := time.Date(2026, time.August, 26, 8, 30, 0, 0, time.UTC)
	pkg := casefile.ReleasePackage{
		ID: "package-concurrent-download", CaseID: caseID, CaseVersion: 1,
		NormalizedText: "[0001] 受访者：可公开的社区记忆",
		VersionSummary: "并发下载缓存复现包", ApprovedBy: "开放负责人", SealedAt: sealedAt,
	}
	pkg.Digest, err = casefile.PackageDigest(pkg)
	if err != nil {
		t.Fatal(err)
	}
	sealedCase := &casefile.Case{
		ID: caseID, Title: "并发下载复现", Status: casefile.StatusSealed, Version: 1,
		CreatedAt: sealedAt, UpdatedAt: sealedAt, Package: &pkg,
		Timeline: []casefile.TimelineEvent{{
			Sequence: 1, Type: "case.sealed", ToStatus: casefile.StatusSealed,
			Actor: "开放负责人", CaseVersion: 1, At: sealedAt,
		}},
	}
	if _, _, err := repository.Create(sealedCase, "case.seed", "seed", sealedAt); err != nil {
		t.Fatal(err)
	}

	handler := web.New(release.NewService(repository, policy.New())).Handler()
	const requests = 24
	start := make(chan struct{})
	errorsFound := make(chan error, requests)
	var ready sync.WaitGroup
	var finished sync.WaitGroup
	ready.Add(requests)
	finished.Add(requests)
	for i := 0; i < requests; i++ {
		go func() {
			defer finished.Done()
			ready.Done()
			<-start
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/cases/"+caseID+"/package/download", nil)
			handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				errorsFound <- fmt.Errorf("并发下载返回状态 %d: %s", recorder.Code, recorder.Body.String())
			}
		}()
	}
	ready.Wait()
	close(start)
	finished.Wait()
	close(errorsFound)
	for requestErr := range errorsFound {
		t.Error(requestErr)
	}
}
