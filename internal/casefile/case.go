package casefile

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

type NewCaseInput struct {
	ID               string
	Title            string
	InterviewDate    string
	IntendedUse      string
	ConsentScope     []string
	RestrictionTerms []string
	Actor            string
	Now              time.Time
}

func NewCase(in NewCaseInput) (*Case, error) {
	if strings.TrimSpace(in.ID) == "" {
		return nil, fieldError("id", "案件 ID 不能为空")
	}
	boundary, err := normalizeBoundary(BoundaryInput{
		Title: in.Title, InterviewDate: in.InterviewDate, IntendedUse: in.IntendedUse,
		ConsentScope: in.ConsentScope, RestrictionTerms: in.RestrictionTerms,
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Actor) == "" {
		return nil, fieldError("actor", "操作者不能为空")
	}
	now := in.Now.UTC()
	c := &Case{
		ID:               strings.TrimSpace(in.ID),
		Title:            boundary.Title,
		InterviewDate:    boundary.InterviewDate,
		IntendedUse:      boundary.IntendedUse,
		ConsentScope:     boundary.ConsentScope,
		RestrictionTerms: boundary.RestrictionTerms,
		Status:           StatusDraft,
		Version:          1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	c.AppendEvent("case.created", "", StatusDraft, in.Actor, "创建开放案", now)
	return c, nil
}

type BoundaryInput struct {
	Title            string
	InterviewDate    string
	IntendedUse      string
	ConsentScope     []string
	RestrictionTerms []string
}

func normalizeBoundary(in BoundaryInput) (BoundaryInput, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.InterviewDate = strings.TrimSpace(in.InterviewDate)
	in.IntendedUse = strings.TrimSpace(in.IntendedUse)
	in.ConsentScope = cleanStrings(in.ConsentScope)
	in.RestrictionTerms = cleanStrings(in.RestrictionTerms)
	if in.Title == "" {
		return BoundaryInput{}, fieldError("title", "案件标题不能为空")
	}
	if len([]rune(in.Title)) > 120 {
		return BoundaryInput{}, fieldError("title", "案件标题不能超过 120 个字符")
	}
	if in.InterviewDate == "" {
		return BoundaryInput{}, fieldError("interviewDate", "访谈日期不能为空")
	}
	if _, err := time.Parse("2006-01-02", in.InterviewDate); err != nil {
		return BoundaryInput{}, fieldError("interviewDate", "访谈日期必须采用 YYYY-MM-DD")
	}
	if in.IntendedUse == "" {
		return BoundaryInput{}, fieldError("intendedUse", "拟开放用途不能为空")
	}
	if len(in.ConsentScope) == 0 {
		return BoundaryInput{}, fieldError("consentScope", "至少登记一项授权范围")
	}
	return in, nil
}

func (c *Case) ReviseBoundary(in BoundaryInput, actor string, now time.Time) error {
	if c.Status != StatusDraft {
		return StateError{Status: c.Status, Operation: "修订案件边界"}
	}
	if strings.TrimSpace(actor) == "" {
		return fieldError("actor", "操作者不能为空")
	}
	normalized, err := normalizeBoundary(in)
	if err != nil {
		return err
	}
	changed := make([]string, 0, 5)
	if c.Title != normalized.Title {
		changed = append(changed, "案件标题")
	}
	if c.InterviewDate != normalized.InterviewDate {
		changed = append(changed, "访谈日期")
	}
	if c.IntendedUse != normalized.IntendedUse {
		changed = append(changed, "拟开放用途")
	}
	if !reflect.DeepEqual(c.ConsentScope, normalized.ConsentScope) {
		changed = append(changed, "授权范围")
	}
	if !reflect.DeepEqual(c.RestrictionTerms, normalized.RestrictionTerms) {
		changed = append(changed, "限制条款")
	}
	if len(changed) == 0 {
		return fieldError("boundary", "案件边界未发生实际变化")
	}
	c.Title = normalized.Title
	c.InterviewDate = normalized.InterviewDate
	c.IntendedUse = normalized.IntendedUse
	c.ConsentScope = normalized.ConsentScope
	c.RestrictionTerms = normalized.RestrictionTerms
	c.Touch(now)
	c.AppendEvent("case.boundary_revised", c.Status, c.Status, actor, fmt.Sprintf("修订字段：%s", strings.Join(changed, "、")), now)
	return nil
}

func cleanStrings(values []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func (c *Case) AppendEvent(kind string, from, to Status, actor, reason string, at time.Time) {
	c.Timeline = append(c.Timeline, TimelineEvent{
		Sequence:    int64(len(c.Timeline) + 1),
		Type:        kind,
		FromStatus:  from,
		ToStatus:    to,
		Actor:       actor,
		Reason:      reason,
		CaseVersion: c.Version,
		At:          at.UTC(),
	})
}

func (c *Case) Touch(now time.Time) {
	c.UpdatedAt = now.UTC()
	c.Preview = nil
}

func (c *Case) Transition(to Status, kind, actor, reason string, now time.Time) {
	from := c.Status
	c.Status = to
	c.UpdatedAt = now.UTC()
	c.AppendEvent(kind, from, to, actor, reason, now)
}

func (c *Case) Clone() *Case {
	if c == nil {
		return nil
	}
	copyCase := *c
	copyCase.ConsentScope = append([]string(nil), c.ConsentScope...)
	copyCase.RestrictionTerms = append([]string(nil), c.RestrictionTerms...)
	copyCase.Segments = append([]TranscriptSegment(nil), c.Segments...)
	for i := range copyCase.Segments {
		copyCase.Segments[i].SensitivityTags = append([]string(nil), c.Segments[i].SensitivityTags...)
	}
	copyCase.Findings = append([]PolicyFinding(nil), c.Findings...)
	copyCase.Decisions = append([]RedactionDecision(nil), c.Decisions...)
	copyCase.Reviews = append([]ReviewRecord(nil), c.Reviews...)
	for i := range copyCase.Reviews {
		copyCase.Reviews[i].AffectedSegmentIDs = append([]string(nil), c.Reviews[i].AffectedSegmentIDs...)
		copyCase.Reviews[i].Items = append([]ReviewItem(nil), c.Reviews[i].Items...)
	}
	copyCase.Timeline = append([]TimelineEvent(nil), c.Timeline...)
	if c.Preview != nil {
		preview := *c.Preview
		preview.Segments = append([]PreviewSegment(nil), c.Preview.Segments...)
		copyCase.Preview = &preview
	}
	if c.Package != nil {
		pkg := *c.Package
		pkg.DecisionSnapshot = append([]DecisionSnapshot(nil), c.Package.DecisionSnapshot...)
		pkg.ReviewSnapshot = append([]ReviewRecord(nil), c.Package.ReviewSnapshot...)
		copyCase.Package = &pkg
	}
	return &copyCase
}
