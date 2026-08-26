package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"oral-history-release-desk/internal/casefile"
)

func TestFileStorePersistsAndReplaysFirstIdempotentResult(t *testing.T) {
	directory := t.TempDir()
	repository, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	c, err := casefile.NewCase(casefile.NewCaseInput{ID: "case-store", Title: "账本测试", InterviewDate: "2025-01-01", IntendedUse: "研究", ConsentScope: []string{"研究"}, Actor: "甲", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	created, replayed, err := repository.Create(c, "case.create", "key-create", now)
	if err != nil || replayed {
		t.Fatalf("首次创建失败: replayed=%v err=%v", replayed, err)
	}
	first, replayed, err := repository.Execute(created.ID, created.Version, "segment.add", "key-segment", now, func(working *casefile.Case) error {
		return working.AddSegment(casefile.SegmentInput{ID: "s1", Sequence: 1, StartMillis: 0, EndMillis: 1000, OriginalText: "文本", SpeakerLabel: "甲"}, "甲", now)
	})
	if err != nil || replayed || first.Version != 2 {
		t.Fatalf("首次写入失败: %#v %v", first, err)
	}
	retry, replayed, err := repository.Execute(created.ID, created.Version, "segment.add", "key-segment", now, func(*casefile.Case) error {
		t.Fatal("幂等重试不应再次执行变更函数")
		return nil
	})
	if err != nil || !replayed || retry.Version != first.Version {
		t.Fatalf("幂等重放错误: replayed=%v version=%d err=%v", replayed, retry.Version, err)
	}
	reopened, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reopened.Get(created.ID)
	if err != nil || loaded.Version != 2 || len(loaded.Segments) != 1 {
		t.Fatalf("恢复结果错误: %#v %v", loaded, err)
	}
}

func TestFileStoreRejectsCorruptLedger(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "ledger.json"), []byte(`{"schemaVersion":1,"ledgerSequence":`), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(directory); err == nil {
		t.Fatal("截断账本必须拒绝启动")
	}
}
