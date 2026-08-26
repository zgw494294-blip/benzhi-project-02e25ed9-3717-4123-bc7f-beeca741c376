package casefile

import (
	"testing"
	"time"
)

func TestReturnedCaseOnlyAllowsAffectedSegment(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	c, err := NewCase(NewCaseInput{ID: "case-1", Title: "测试案件", InterviewDate: "2025-01-01", IntendedUse: "公开研究", ConsentScope: []string{"公开"}, Actor: "整理员", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	for _, segment := range []SegmentInput{
		{ID: "s1", Sequence: 1, StartMillis: 0, EndMillis: 1000, OriginalText: "第一段", SpeakerLabel: "甲"},
		{ID: "s2", Sequence: 2, StartMillis: 1000, EndMillis: 2000, OriginalText: "第二段", SpeakerLabel: "乙"},
	} {
		if err := c.AddSegment(segment, "整理员", now); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Freeze("整理员", now); err != nil {
		t.Fatal(err)
	}
	c.Status = StatusPendingReview
	c.Preview = &Preview{CaseVersion: c.Version, Text: "预览"}
	err = c.Review(ReviewInput{ID: "r1", Reviewer: "复核员", Outcome: ReviewReturned, ReasonCode: "CONTEXT", Reason: "补充上下文", AffectedSegmentIDs: []string{"s2"}, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if c.Segment("s1").NeedsRecheck || !c.Segment("s2").NeedsRecheck {
		t.Fatal("退回范围未准确映射到片段")
	}
	err = c.ReviseReturnedSegment(SegmentInput{ID: "s1", Sequence: 1, StartMillis: 0, EndMillis: 1000, OriginalText: "非法修改", SpeakerLabel: "甲"}, "整理员", now)
	if err == nil {
		t.Fatal("未受影响片段不应允许修改")
	}
}

func TestSegmentRangesAndSequenceAreValidated(t *testing.T) {
	now := time.Now()
	c, err := NewCase(NewCaseInput{ID: "case-2", Title: "顺序测试", InterviewDate: "2025-01-01", IntendedUse: "研究", ConsentScope: []string{"研究"}, Actor: "甲", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddSegment(SegmentInput{ID: "s1", Sequence: 1, StartMillis: 0, EndMillis: 2000, OriginalText: "文本", SpeakerLabel: "甲"}, "甲", now); err != nil {
		t.Fatal(err)
	}
	if err := c.AddSegment(SegmentInput{ID: "s2", Sequence: 2, StartMillis: 1500, EndMillis: 2500, OriginalText: "重叠", SpeakerLabel: "乙"}, "甲", now); err == nil {
		t.Fatal("重叠时间范围应被拒绝")
	}
	if err := c.AddSegment(SegmentInput{ID: "s3", Sequence: 3, StartMillis: 2000, EndMillis: 3000, OriginalText: "跳号", SpeakerLabel: "乙"}, "甲", now); err != nil {
		t.Fatal(err)
	}
	if err := c.Freeze("甲", now); err == nil {
		t.Fatal("不连续片段顺序应阻止冻结")
	}
}
