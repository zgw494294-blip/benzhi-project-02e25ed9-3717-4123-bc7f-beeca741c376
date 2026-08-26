package casefile

import (
	"strings"
	"time"
)

type ReviewInput struct {
	ID                 string
	Reviewer           string
	Outcome            ReviewOutcome
	ReasonCode         string
	Reason             string
	AffectedSegmentIDs []string
	Items              []ReviewItem
	Now                time.Time
}

func (c *Case) Review(in ReviewInput) error {
	if c.Status != StatusPendingReview {
		return StateError{Status: c.Status, Operation: "作出复核决定"}
	}
	if c.Preview == nil || c.Preview.CaseVersion != c.Version {
		return fieldError("preview", "发布预览不存在或已过期")
	}
	if strings.TrimSpace(in.ID) == "" || strings.TrimSpace(in.Reviewer) == "" {
		return fieldError("review", "复核 ID 和复核员不能为空")
	}
	if in.Outcome != ReviewApproved && in.Outcome != ReviewReturned {
		return fieldError("outcome", "复核结论必须为 approved 或 returned")
	}
	active := c.activeReviewFindingIDs()
	confirmed := make(map[string]bool)
	for _, item := range in.Items {
		if item.Confirmed {
			confirmed[item.FindingID] = true
		}
	}
	if in.Outcome == ReviewApproved {
		for findingID := range active {
			if !confirmed[findingID] {
				return fieldError("items", "批准前必须逐项确认所有有效处置和警告")
			}
		}
	}
	if in.Outcome == ReviewReturned {
		if strings.TrimSpace(in.ReasonCode) == "" || strings.TrimSpace(in.Reason) == "" {
			return fieldError("reason", "退回必须提供结构化理由代码和说明")
		}
		if len(in.AffectedSegmentIDs) == 0 {
			return fieldError("affectedSegmentIds", "退回必须指定至少一个受影响片段")
		}
		seen := make(map[string]bool)
		for _, id := range in.AffectedSegmentIDs {
			segment := c.Segment(id)
			if segment == nil {
				return fieldError("affectedSegmentIds", "受影响片段不存在: "+id)
			}
			if !seen[id] {
				segment.NeedsRecheck = true
				seen[id] = true
			}
		}
	}
	record := ReviewRecord{
		ID:                 strings.TrimSpace(in.ID),
		Reviewer:           strings.TrimSpace(in.Reviewer),
		Outcome:            in.Outcome,
		ReasonCode:         strings.TrimSpace(in.ReasonCode),
		Reason:             strings.TrimSpace(in.Reason),
		AffectedSegmentIDs: cleanStrings(in.AffectedSegmentIDs),
		Items:              append([]ReviewItem(nil), in.Items...),
		CaseVersion:        c.Version + 1,
		CreatedAt:          in.Now.UTC(),
	}
	c.Reviews = append(c.Reviews, record)
	if in.Outcome == ReviewApproved {
		c.Transition(StatusPendingApproval, "review.approved", in.Reviewer, "独立复核通过", in.Now)
	} else {
		c.Preview = nil
		c.Transition(StatusReturned, "review.returned", in.Reviewer, in.ReasonCode+": "+in.Reason, in.Now)
	}
	return nil
}

func (c *Case) activeReviewFindingIDs() map[string]bool {
	result := make(map[string]bool)
	for _, finding := range c.Findings {
		if finding.ResolutionStatus == ResolutionObsolete || finding.Severity == SeverityPass {
			continue
		}
		result[finding.ID] = true
	}
	return result
}

func (c *Case) Seal(pkg ReleasePackage, actor string, now time.Time) error {
	if c.Status != StatusPendingApproval {
		return StateError{Status: c.Status, Operation: "最终批准"}
	}
	if strings.TrimSpace(actor) == "" {
		return fieldError("approvedBy", "开放负责人不能为空")
	}
	if c.Package != nil {
		return fieldError("releasePackage", "发布包已经封存")
	}
	c.Package = &pkg
	c.Transition(StatusSealed, "package.sealed", actor, "最终批准并封存发布包", now)
	return nil
}
