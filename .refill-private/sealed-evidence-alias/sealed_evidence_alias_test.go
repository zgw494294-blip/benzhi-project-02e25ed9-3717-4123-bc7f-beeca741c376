package sealedevidencealias_test

import (
	"testing"

	"oral-history-release-desk/internal/casefile"
	"oral-history-release-desk/internal/policy"
	"oral-history-release-desk/internal/release"
	"oral-history-release-desk/internal/store"
)

func TestEvidenceCatalogDoesNotMutateSealedPackage(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := release.NewService(repository, policy.New())
	created, err := service.CreateCase(release.CreateCaseInput{
		ID: "case-sealed-alias", Title: "封存证据所有权", InterviewDate: "2025-08-01",
		IntendedUse: "研究", ConsentScope: []string{"研究"}, RestrictionTerms: []string{"不得披露姓名"},
		Actor: "整理员", IdempotencyKey: "create",
	})
	if err != nil {
		t.Fatal(err)
	}
	c := created.Case
	command, commandErr := service.AddSegment(c.ID, release.AddSegmentInput{
		CommandMeta: meta(c, "segment-1", "整理员"), ID: "s1", Sequence: 1,
		StartMillis: 0, EndMillis: 1000, OriginalText: "第一段社区记忆", SpeakerLabel: "甲", SensitivityTags: []string{"姓名"},
	})
	c = mustCommand(t, command, commandErr)
	command, commandErr = service.AddSegment(c.ID, release.AddSegmentInput{
		CommandMeta: meta(c, "segment-2", "整理员"), ID: "s2", Sequence: 2,
		StartMillis: 1000, EndMillis: 2000, OriginalText: "第二段社区记忆", SpeakerLabel: "乙", SensitivityTags: []string{"姓名"},
	})
	c = mustCommand(t, command, commandErr)
	command, commandErr = service.Freeze(c.ID, meta(c, "freeze", "整理员"))
	c = mustCommand(t, command, commandErr)
	command, commandErr = service.RunCheck(c.ID, meta(c, "check", "整理员"))
	c = mustCommand(t, command, commandErr)
	c = resolveBlocks(t, service, c, "decide-initial")
	command, commandErr = service.GeneratePreview(c.ID, meta(c, "preview-1", "整理员"))
	c = mustCommand(t, command, commandErr)
	command, commandErr = service.SubmitReview(c.ID, meta(c, "submit-1", "整理员"))
	c = mustCommand(t, command, commandErr)
	returned, err := service.ReviewDecision(c.ID, release.ReviewDecisionInput{
		CommandMeta: meta(c, "return", "复核员"), Outcome: casefile.ReviewReturned,
		ReasonCode: "CONTEXT", Reason: "补充分段上下文", AffectedSegmentIDs: []string{"s2", "s1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	c = returned.Case
	command, commandErr = service.ReviseSegment(c.ID, release.ReviseSegmentInput{
		CommandMeta: meta(c, "revise-2", "整理员"), ID: "s2", Sequence: 2,
		StartMillis: 1000, EndMillis: 2000, OriginalText: "第二段社区记忆（补充）", SpeakerLabel: "乙", SensitivityTags: []string{"姓名"},
	})
	c = mustCommand(t, command, commandErr)
	command, commandErr = service.ReviseSegment(c.ID, release.ReviseSegmentInput{
		CommandMeta: meta(c, "revise-1", "整理员"), ID: "s1", Sequence: 1,
		StartMillis: 0, EndMillis: 1000, OriginalText: "第一段社区记忆（补充）", SpeakerLabel: "甲", SensitivityTags: []string{"姓名"},
	})
	c = mustCommand(t, command, commandErr)
	command, commandErr = service.RunTargetedCheck(c.ID, meta(c, "recheck", "整理员"))
	c = mustCommand(t, command, commandErr)
	c = resolveBlocks(t, service, c, "decide-targeted")
	command, commandErr = service.GeneratePreview(c.ID, meta(c, "preview-2", "整理员"))
	c = mustCommand(t, command, commandErr)
	command, commandErr = service.SubmitReview(c.ID, meta(c, "submit-2", "整理员"))
	c = mustCommand(t, command, commandErr)
	approved, err := service.ReviewDecision(c.ID, release.ReviewDecisionInput{
		CommandMeta: meta(c, "approve-review", "复核员"), Outcome: casefile.ReviewApproved,
		Items: activeItems(c),
	})
	if err != nil {
		t.Fatal(err)
	}
	c = approved.Case
	if _, err := service.FinalApprove(c.ID, meta(c, "seal", "负责人")); err != nil {
		t.Fatal(err)
	}
	before, err := service.VerifyPackage(c.ID)
	if err != nil || !before.Valid {
		t.Fatalf("封存后的初始摘要无效: %#v %v", before, err)
	}
	if _, err := service.EvidenceCatalog(c.ID, release.EvidenceFilter{}); err != nil {
		t.Fatal(err)
	}
	after, err := service.VerifyPackage(c.ID)
	if err != nil || !after.Valid {
		t.Fatalf("读取证据目录污染了不可变发布包: %#v %v", after, err)
	}
}

func resolveBlocks(t *testing.T, service *release.Service, c *casefile.Case, key string) *casefile.Case {
	t.Helper()
	blocks := c.OpenBlocks()
	items := make([]release.BatchDecisionItem, 0, len(blocks))
	for _, finding := range blocks {
		items = append(items, release.BatchDecisionItem{
			FindingID: finding.ID, Action: casefile.ActionRedact, Rationale: "保护身份",
		})
	}
	result, err := service.DecideFindings(c.ID, release.DecideFindingsInput{
		CommandMeta: meta(c, key, "整理员"), Decisions: items,
	})
	return mustCommand(t, result, err)
}

func activeItems(c *casefile.Case) []casefile.ReviewItem {
	items := make([]casefile.ReviewItem, 0)
	for _, finding := range c.Findings {
		if finding.ResolutionStatus != casefile.ResolutionObsolete && finding.Severity != casefile.SeverityPass {
			items = append(items, casefile.ReviewItem{FindingID: finding.ID, Confirmed: true})
		}
	}
	return items
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
