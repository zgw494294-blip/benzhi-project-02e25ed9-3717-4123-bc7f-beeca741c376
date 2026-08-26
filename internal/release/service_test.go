package release

import (
	"bytes"
	"errors"
	"testing"

	"oral-history-release-desk/internal/casefile"
	"oral-history-release-desk/internal/policy"
	"oral-history-release-desk/internal/store"
)

func TestCompleteWorkflowWithReturnAndTargetedRecheck(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(repository, policy.New())
	created, err := service.CreateCase(CreateCaseInput{ID: "case-flow", Title: "完整流程", InterviewDate: "2025-06-01", IntendedUse: "公开研究", ConsentScope: []string{"公开研究"}, RestrictionTerms: []string{"不得披露姓名"}, Actor: "整理员", IdempotencyKey: "01"})
	if err != nil {
		t.Fatal(err)
	}
	c := created.Case
	added, err := service.AddSegment(c.ID, AddSegmentInput{CommandMeta: meta(c, "02", "整理员"), ID: "s1", Sequence: 1, StartMillis: 0, EndMillis: 5000, OriginalText: "王某住在北街", SpeakerLabel: "王某", SensitivityTags: []string{"姓名"}})
	if err != nil {
		t.Fatal(err)
	}
	c = added.Case
	added, err = service.AddSegment(c.ID, AddSegmentInput{CommandMeta: meta(c, "02b", "整理员"), ID: "s2", Sequence: 2, StartMillis: 5000, EndMillis: 9000, OriginalText: "这是一段可公开的社区记忆", SpeakerLabel: "受访者"})
	if err != nil {
		t.Fatal(err)
	}
	c = added.Case
	command, commandErr := service.Freeze(c.ID, meta(c, "03", "整理员"))
	c = mustCommand(t, command, commandErr)
	command, commandErr = service.RunCheck(c.ID, meta(c, "04", "整理员"))
	c = mustCommand(t, command, commandErr)
	for index, finding := range c.OpenBlocks() {
		result, err := service.DecideFinding(c.ID, DecideFindingInput{CommandMeta: meta(c, string(rune('a'+index)), "整理员"), FindingID: finding.ID, Action: casefile.ActionRedact, Rationale: "保护身份"})
		if err != nil {
			t.Fatal(err)
		}
		c = result.Case
	}
	command, commandErr = service.GeneratePreview(c.ID, meta(c, "05", "整理员"))
	c = mustCommand(t, command, commandErr)
	command, commandErr = service.SubmitReview(c.ID, meta(c, "06", "整理员"))
	c = mustCommand(t, command, commandErr)
	returned, err := service.ReviewDecision(c.ID, ReviewDecisionInput{CommandMeta: meta(c, "07", "复核员"), Outcome: casefile.ReviewReturned, ReasonCode: "CONTEXT", Reason: "替换为不含身份的概括", AffectedSegmentIDs: []string{"s1"}})
	if err != nil {
		t.Fatal(err)
	}
	c = returned.Case
	revised, err := service.ReviseSegment(c.ID, ReviseSegmentInput{CommandMeta: meta(c, "08", "整理员"), ID: "s1", Sequence: 1, StartMillis: 0, EndMillis: 5000, OriginalText: "受访者曾居住在老城区", SpeakerLabel: "受访者", SensitivityTags: nil})
	if err != nil {
		t.Fatal(err)
	}
	c = revised.Case
	command, commandErr = service.RunTargetedCheck(c.ID, meta(c, "09", "整理员"))
	c = mustCommand(t, command, commandErr)
	summary, err := service.RiskSummary(c.ID, RiskSummaryFilter{})
	if err != nil {
		t.Fatal(err)
	}
	changes := make(map[string]string)
	for _, item := range summary.Differences {
		changes[item.SegmentID+"/"+item.RuleCode] = item.Status
	}
	if changes["s1/SENSITIVE_PERSONAL_DATA"] != "removed" || changes["s1/SEGMENT_CLEAR"] != "added" || changes["s2/SEGMENT_CLEAR"] != "unchanged" {
		t.Fatalf("定向差异不完整: %#v", changes)
	}
	command, commandErr = service.GeneratePreview(c.ID, meta(c, "10", "整理员"))
	c = mustCommand(t, command, commandErr)
	command, commandErr = service.SubmitReview(c.ID, meta(c, "11", "整理员"))
	c = mustCommand(t, command, commandErr)
	items := activeItems(c)
	approved, err := service.ReviewDecision(c.ID, ReviewDecisionInput{CommandMeta: meta(c, "12", "复核员"), Outcome: casefile.ReviewApproved, Items: items})
	if err != nil {
		t.Fatal(err)
	}
	c = approved.Case
	sealed, err := service.FinalApprove(c.ID, meta(c, "13", "负责人"))
	if err != nil {
		t.Fatal(err)
	}
	if sealed.Case.Status != casefile.StatusSealed || sealed.Package == nil {
		t.Fatal("案件未完成封存")
	}
	verification, err := service.VerifyPackage(c.ID)
	if err != nil || !verification.Valid {
		t.Fatalf("发布包摘要校验失败: %#v %v", verification, err)
	}
	catalog, err := service.EvidenceCatalog(c.ID, EvidenceFilter{Outcome: casefile.ReviewApproved})
	if err != nil || catalog.Counts.NormalizedTexts != 1 || catalog.Counts.Reviews != 1 || !catalog.Checksum.Valid {
		t.Fatalf("证据目录无效: %#v %v", catalog, err)
	}
	name1, download1, err := service.PackageDownload(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	name2, download2, err := service.PackageDownload(c.ID)
	if err != nil || name1 != name2 || !bytes.Equal(download1, download2) || !bytes.Contains(download1, []byte(sealed.Package.Digest)) {
		t.Fatalf("规范化下载不稳定: %s %s %v", name1, name2, err)
	}
}

func TestDraftBoundaryAndBatchSegmentsAreAtomic(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(repository, policy.New())
	created, err := service.CreateCase(CreateCaseInput{ID: "case-batch", Title: "原始标题", InterviewDate: "2025-01-02", IntendedUse: "研究", ConsentScope: []string{"研究"}, Actor: "整理员", IdempotencyKey: "create"})
	if err != nil {
		t.Fatal(err)
	}
	c := created.Case
	batched, err := service.AddSegments(c.ID, AddSegmentsInput{CommandMeta: meta(c, "batch", "整理员"), Segments: []BatchSegmentItem{
		{Sequence: 1, StartMillis: 0, EndMillis: 1000, OriginalText: "第一段", SpeakerLabel: "甲", SensitivityTags: []string{" 姓名 ", "姓名"}},
		{Sequence: 2, StartMillis: 1000, EndMillis: 2000, OriginalText: "第二段", SpeakerLabel: "乙"},
		{Sequence: 3, StartMillis: 2000, EndMillis: 3000, OriginalText: "第三段", SpeakerLabel: "丙"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if batched.Case.Version != c.Version+1 || len(batched.Case.Segments) != 3 || len(batched.Case.Segments[0].SensitivityTags) != 1 {
		t.Fatalf("批量登记结果无效: %#v", batched.Case)
	}
	replayed, err := service.AddSegments(c.ID, AddSegmentsInput{CommandMeta: meta(c, "batch", "整理员"), Segments: []BatchSegmentItem{{Sequence: 99}}})
	if err != nil || !replayed.Idempotent || len(replayed.Case.Segments) != 3 {
		t.Fatalf("批量登记幂等重放失败: %#v %v", replayed, err)
	}
	c = batched.Case
	revised, err := service.ReviseBoundary(c.ID, ReviseBoundaryInput{CommandMeta: meta(c, "revise", "整理员"), Title: "修订标题", InterviewDate: "2025-01-02", IntendedUse: "公开研究", ConsentScope: []string{" 公开研究 ", "公开研究", "教育"}, RestrictionTerms: []string{"匿名", " 匿名 "}})
	if err != nil {
		t.Fatal(err)
	}
	if revised.Case.Version != c.Version+1 || len(revised.Case.Segments) != 3 || len(revised.Case.ConsentScope) != 2 || revised.Case.Timeline[len(revised.Case.Timeline)-1].Reason != "修订字段：案件标题、拟开放用途、授权范围、限制条款" {
		t.Fatalf("案件边界修订结果无效: %#v", revised.Case)
	}
	_, err = service.ReviseBoundary(revised.Case.ID, ReviseBoundaryInput{CommandMeta: meta(revised.Case, "empty", "整理员"), Title: revised.Case.Title, InterviewDate: revised.Case.InterviewDate, IntendedUse: revised.Case.IntendedUse, ConsentScope: revised.Case.ConsentScope, RestrictionTerms: revised.Case.RestrictionTerms})
	if err == nil {
		t.Fatal("未变化的边界修订应被拒绝")
	}
}

func TestBatchValidationAndDecisionConflictDoNotPartiallyCommit(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(repository, policy.New())
	created, err := service.CreateCase(CreateCaseInput{ID: "case-conflict", Title: "批量冲突", InterviewDate: "2025-03-02", IntendedUse: "公开研究", ConsentScope: []string{"公开研究"}, RestrictionTerms: []string{"不得披露姓名"}, Actor: "整理员", IdempotencyKey: "create"})
	if err != nil {
		t.Fatal(err)
	}
	c := created.Case
	added, err := service.AddSegment(c.ID, AddSegmentInput{CommandMeta: meta(c, "one", "整理员"), ID: "s1", Sequence: 1, StartMillis: 0, EndMillis: 1000, OriginalText: "张某的经历", SpeakerLabel: "张某", SensitivityTags: []string{"姓名"}})
	if err != nil {
		t.Fatal(err)
	}
	c = added.Case
	_, err = service.AddSegments(c.ID, AddSegmentsInput{CommandMeta: meta(c, "bad-batch", "整理员"), Segments: []BatchSegmentItem{
		{Sequence: 2, StartMillis: 500, EndMillis: 1500, OriginalText: "重叠", SpeakerLabel: "乙"},
		{Sequence: 1, StartMillis: 2000, EndMillis: 3000, OriginalText: "重复顺序", SpeakerLabel: "丙"},
	}})
	var validation casefile.MultiValidationError
	if !errors.As(err, &validation) || validation.Fields["segments[0].timeRange"] == "" || validation.Fields["segments[1].sequence"] == "" {
		t.Fatalf("未返回全部行错误: %#v %v", validation, err)
	}
	unchanged, _ := service.Get(c.ID)
	if unchanged.Version != c.Version || len(unchanged.Segments) != 1 || len(unchanged.Timeline) != len(c.Timeline) {
		t.Fatal("失败批次改变了案件")
	}
	command, commandErr := service.Freeze(c.ID, meta(c, "freeze", "整理员"))
	c = mustCommand(t, command, commandErr)
	command, commandErr = service.RunCheck(c.ID, meta(c, "check", "整理员"))
	c = mustCommand(t, command, commandErr)
	blocks := c.OpenBlocks()
	if len(blocks) < 2 {
		t.Fatalf("预期同片段至少两个阻断项: %#v", blocks)
	}
	beforeVersion := c.Version
	_, err = service.DecideFindings(c.ID, DecideFindingsInput{CommandMeta: meta(c, "conflicting", "整理员"), Decisions: []BatchDecisionItem{
		{FindingID: blocks[0].ID, Action: casefile.ActionReplace, ReplacementText: "替代甲", Rationale: "保护身份"},
		{FindingID: blocks[1].ID, Action: casefile.ActionReplace, ReplacementText: "替代乙", Rationale: "保护身份"},
	}})
	var conflict casefile.DecisionConflictError
	if !errors.As(err, &conflict) || len(conflict.Conflicts) != 1 || len(conflict.Conflicts[0].FindingIDs) != 2 {
		t.Fatalf("未返回同片段候选冲突: %#v %v", conflict, err)
	}
	unchanged, _ = service.Get(c.ID)
	if unchanged.Version != beforeVersion || len(unchanged.Decisions) != 0 {
		t.Fatal("冲突批次改变了处置决定")
	}
	resolved, err := service.DecideFindings(c.ID, DecideFindingsInput{CommandMeta: meta(c, "unified", "整理员"), Decisions: []BatchDecisionItem{
		{FindingID: blocks[0].ID, Action: casefile.ActionRedact, Rationale: "保护身份"},
		{FindingID: blocks[1].ID, Action: casefile.ActionRedact, Rationale: "保护身份"},
	}})
	if err != nil || resolved.Case.Version != beforeVersion+1 || len(resolved.Case.OpenBlocks()) != 0 {
		t.Fatalf("统一候选结果未能原子闭合: %#v %v", resolved, err)
	}
}

func meta(c *casefile.Case, key, actor string) CommandMeta {
	return CommandMeta{ExpectedVersion: c.Version, IdempotencyKey: key, Actor: actor}
}

func mustCommand(t *testing.T, result CommandResult, err error) *casefile.Case {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	return result.Case
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
