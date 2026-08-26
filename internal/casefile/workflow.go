package casefile

import (
	"strings"
	"time"
)

func (c *Case) Freeze(actor string, now time.Time) error {
	if c.Status != StatusDraft {
		return StateError{Status: c.Status, Operation: "冻结案件"}
	}
	if len(c.Segments) == 0 {
		return fieldError("segments", "至少登记一个转录片段后才能冻结")
	}
	for i, segment := range c.Segments {
		if segment.Sequence != i+1 {
			return fieldError("segments", "片段顺序必须从 1 开始连续排列")
		}
	}
	c.Transition(StatusPendingCheck, "case.frozen", actor, "冻结转录与授权边界", now)
	return nil
}

func (c *Case) ApplyFindings(runID string, findings []PolicyFinding, targeted bool, actor string, now time.Time) error {
	if !targeted && c.Status != StatusPendingCheck && c.Status != StatusRemediation {
		return StateError{Status: c.Status, Operation: "全量校核"}
	}
	if targeted && c.Status != StatusReturned {
		return StateError{Status: c.Status, Operation: "定向重新校核"}
	}
	if strings.TrimSpace(runID) == "" {
		return fieldError("runId", "校核运行 ID 不能为空")
	}
	if targeted {
		for i := range c.Findings {
			if segment := c.Segment(c.Findings[i].SegmentID); segment != nil && segment.NeedsRecheck {
				c.Findings[i].ResolutionStatus = ResolutionObsolete
			}
		}
		c.Findings = append(c.Findings, findings...)
		for i := range c.Segments {
			if c.Segments[i].NeedsRecheck {
				c.Segments[i].NeedsRecheck = false
			}
		}
	} else {
		c.Findings = findings
		c.Decisions = nil
	}
	c.LastRunID = runID
	c.Preview = nil
	to := StatusPendingReview
	if c.HasOpenBlocks() {
		to = StatusRemediation
	}
	kind := "policy.checked"
	reason := "完成确定性全量校核"
	if targeted {
		kind = "policy.rechecked"
		reason = "完成退回片段的定向重新校核"
	}
	c.Transition(to, kind, actor, reason, now)
	return nil
}

func (c *Case) HasOpenBlocks() bool {
	for _, finding := range c.Findings {
		if finding.Severity == SeverityBlock && finding.ResolutionStatus == ResolutionOpen {
			return true
		}
	}
	return false
}

func (c *Case) OpenBlocks() []PolicyFinding {
	result := make([]PolicyFinding, 0)
	for _, finding := range c.Findings {
		if finding.Severity == SeverityBlock && finding.ResolutionStatus == ResolutionOpen {
			result = append(result, finding)
		}
	}
	return result
}

func (c *Case) SetPreview(preview Preview, actor string, now time.Time) error {
	if c.Status != StatusRemediation && c.Status != StatusPendingReview {
		return StateError{Status: c.Status, Operation: "生成发布预览"}
	}
	if c.HasOpenBlocks() {
		return fieldError("findings", "仍有阻断项未闭合")
	}
	preview.CaseVersion = c.Version + 1
	preview.GeneratedAt = now.UTC()
	c.Preview = &preview
	if c.Status == StatusRemediation {
		c.Transition(StatusPendingReview, "preview.generated", actor, "阻断项闭合并生成发布预览", now)
	} else {
		c.UpdatedAt = now.UTC()
		c.AppendEvent("preview.generated", c.Status, c.Status, actor, "重新生成发布预览", now)
	}
	return nil
}

func (c *Case) SubmitReview(actor string, now time.Time) error {
	if c.Status != StatusPendingReview {
		return StateError{Status: c.Status, Operation: "提交独立复核"}
	}
	if c.HasOpenBlocks() {
		return fieldError("findings", "仍有阻断项未闭合")
	}
	if c.Preview == nil || c.Preview.CaseVersion != c.Version {
		return fieldError("preview", "发布预览不存在或已过期，请重新生成")
	}
	if len(c.Timeline) > 0 && c.Timeline[len(c.Timeline)-1].Type == "review.submitted" {
		return StateError{Status: c.Status, Operation: "重复提交同一复核版本"}
	}
	c.UpdatedAt = now.UTC()
	c.Preview.CaseVersion = c.Version + 1
	c.AppendEvent("review.submitted", c.Status, c.Status, actor, "提交独立复核", now)
	return nil
}
