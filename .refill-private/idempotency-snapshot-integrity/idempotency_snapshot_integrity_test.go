package idempotencysnapshot_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"oral-history-release-desk/internal/casefile"
	"oral-history-release-desk/internal/policy"
	"oral-history-release-desk/internal/release"
	"oral-history-release-desk/internal/store"
)

func TestOpenRejectsCorruptIdempotencySnapshots(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{
			name: "invalid historical version",
			mutate: func(result map[string]any) {
				result["version"] = float64(7)
			},
		},
		{
			name: "invalid historical package digest",
			mutate: func(result map[string]any) {
				result["releasePackage"].(map[string]any)["digest"] = "sha256:corrupted"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			writeValidSealedLedger(t, directory)
			path := filepath.Join(directory, "ledger.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var document map[string]any
			if err := json.Unmarshal(data, &document); err != nil {
				t.Fatal(err)
			}
			records := document["idempotency"].(map[string]any)
			record := records["sealed-case\x00approve"].(map[string]any)
			test.mutate(record["result"].(map[string]any))
			corrupt, err := json.Marshal(document)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, corrupt, 0o640); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Open(directory); err == nil {
				t.Fatalf("TestOpenRejectsCorruptIdempotencySnapshots: 启动接受了损坏的历史幂等快照 (%s)", test.name)
			}
		})
	}
}

func writeValidSealedLedger(t *testing.T, directory string) {
	t.Helper()
	repository, err := store.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	service := release.NewService(repository, policy.New())
	created, err := service.CreateCase(release.CreateCaseInput{
		ID: "sealed-case", Title: "已封存案件", InterviewDate: "2026-01-01", IntendedUse: "研究",
		ConsentScope: []string{"研究"}, Actor: "整理员", IdempotencyKey: "create",
	})
	if err != nil {
		t.Fatal(err)
	}
	c := created.Case
	added, err := service.AddSegment(c.ID, release.AddSegmentInput{
		CommandMeta: meta(c, "segment", "整理员"), ID: "s1", Sequence: 1,
		StartMillis: 0, EndMillis: 1000, OriginalText: "可公开文本", SpeakerLabel: "受访者",
	})
	if err != nil {
		t.Fatal(err)
	}
	c = added.Case
	command, commandErr := service.Freeze(c.ID, meta(c, "freeze", "整理员"))
	c = mustCommand(t, command, commandErr)
	command, commandErr = service.RunCheck(c.ID, meta(c, "check", "整理员"))
	c = mustCommand(t, command, commandErr)
	command, commandErr = service.GeneratePreview(c.ID, meta(c, "preview", "整理员"))
	c = mustCommand(t, command, commandErr)
	command, commandErr = service.SubmitReview(c.ID, meta(c, "submit", "整理员"))
	c = mustCommand(t, command, commandErr)
	reviewed, err := service.ReviewDecision(c.ID, release.ReviewDecisionInput{
		CommandMeta: meta(c, "review", "复核员"), Outcome: casefile.ReviewApproved,
	})
	if err != nil {
		t.Fatal(err)
	}
	c = reviewed.Case
	if _, err := service.FinalApprove(c.ID, meta(c, "approve", "负责人")); err != nil {
		t.Fatal(err)
	}
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
