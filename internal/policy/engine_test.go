package policy

import (
	"reflect"
	"testing"
	"time"

	"oral-history-release-desk/internal/casefile"
)

func TestRunIsDeterministicAndPreviewAppliesRedaction(t *testing.T) {
	now := time.Now()
	c, err := casefile.NewCase(casefile.NewCaseInput{ID: "case-policy", Title: "规则测试", InterviewDate: "2025-02-03", IntendedUse: "公开展览", ConsentScope: []string{"内部研究"}, RestrictionTerms: []string{"姓名匿名"}, Actor: "甲", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddSegment(casefile.SegmentInput{ID: "s1", Sequence: 1, StartMillis: 0, EndMillis: 1000, OriginalText: "张某住在北街", SpeakerLabel: "张某", SensitivityTags: []string{"姓名", "住址"}}, "甲", now); err != nil {
		t.Fatal(err)
	}
	engine := New()
	first, err := engine.Run(c, "run-fixed")
	if err != nil {
		t.Fatal(err)
	}
	second, err := engine.Run(c, "run-fixed")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("相同输入应产生完全相同的校核结果")
	}
	if len(first) < 2 || first[0].Severity != casefile.SeverityBlock {
		t.Fatalf("预期多个稳定阻断项，实际 %#v", first)
	}
	c.Findings = first
	for _, item := range first {
		if item.Severity != casefile.SeverityBlock {
			continue
		}
		if err := c.Decide(casefile.DecisionInput{ID: "d-" + item.ID, FindingID: item.ID, Action: casefile.ActionRedact, Rationale: "保护身份", DecidedBy: "甲", Now: now}); err != nil {
			c.Status = casefile.StatusRemediation
			if err := c.Decide(casefile.DecisionInput{ID: "d-" + item.ID, FindingID: item.ID, Action: casefile.ActionRedact, Rationale: "保护身份", DecidedBy: "甲", Now: now}); err != nil {
				t.Fatal(err)
			}
		}
	}
	preview, err := engine.Preview(c)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Segments[0].After != "〔内容已遮蔽〕" {
		t.Fatalf("遮蔽结果错误: %s", preview.Segments[0].After)
	}
}
