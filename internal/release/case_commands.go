package release

import (
	"context"
	"fmt"
	"strings"
	"time"

	"oral-history-release-desk/internal/casefile"
)

type CreateCaseInput struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	InterviewDate    string   `json:"interviewDate"`
	IntendedUse      string   `json:"intendedUse"`
	ConsentScope     []string `json:"consentScope"`
	RestrictionTerms []string `json:"restrictionTerms"`
	Actor            string   `json:"actor"`
	IdempotencyKey   string   `json:"idempotencyKey"`
}

type ReviseBoundaryInput struct {
	CommandMeta
	Title            string   `json:"title"`
	InterviewDate    string   `json:"interviewDate"`
	IntendedUse      string   `json:"intendedUse"`
	ConsentScope     []string `json:"consentScope"`
	RestrictionTerms []string `json:"restrictionTerms"`
}

func (s *Service) ReviseBoundary(caseID string, in ReviseBoundaryInput) (CommandResult, error) {
	return s.execute(caseID, "case.boundary.revise", in.CommandMeta, func(c *casefile.Case, now time.Time) error {
		return c.ReviseBoundary(casefile.BoundaryInput{
			Title: in.Title, InterviewDate: in.InterviewDate, IntendedUse: in.IntendedUse,
			ConsentScope: in.ConsentScope, RestrictionTerms: in.RestrictionTerms,
		}, in.Actor, now)
	})
}

func (s *Service) ReviseBoundaryContext(ctx context.Context, caseID string, in ReviseBoundaryInput) (CommandResult, error) {
	return s.executeContext(ctx, caseID, "case.boundary.revise", in.CommandMeta, func(c *casefile.Case, now time.Time) error {
		return c.ReviseBoundary(casefile.BoundaryInput{
			Title: in.Title, InterviewDate: in.InterviewDate, IntendedUse: in.IntendedUse,
			ConsentScope: in.ConsentScope, RestrictionTerms: in.RestrictionTerms,
		}, in.Actor, now)
	})
}

func (s *Service) CreateCase(in CreateCaseInput) (CommandResult, error) {
	id, err := requiredID(in.ID, "case", s.newID)
	if err != nil {
		return CommandResult{}, err
	}
	now := s.now().UTC()
	c, err := casefile.NewCase(casefile.NewCaseInput{
		ID: id, Title: in.Title, InterviewDate: in.InterviewDate, IntendedUse: in.IntendedUse,
		ConsentScope: in.ConsentScope, RestrictionTerms: in.RestrictionTerms, Actor: in.Actor, Now: now,
	})
	if err != nil {
		return CommandResult{}, err
	}
	if strings.TrimSpace(in.IdempotencyKey) == "" {
		return CommandResult{}, casefile.ValidationError{Field: "idempotencyKey", Message: "幂等键不能为空"}
	}
	result, replayed, err := s.store.Create(c, "case.create", in.IdempotencyKey, now)
	return CommandResult{Case: result, Idempotent: replayed}, err
}

type AddSegmentInput struct {
	CommandMeta
	ID              string   `json:"id"`
	Sequence        int      `json:"sequence"`
	StartMillis     int64    `json:"startMillis"`
	EndMillis       int64    `json:"endMillis"`
	OriginalText    string   `json:"originalText"`
	SpeakerLabel    string   `json:"speakerLabel"`
	SensitivityTags []string `json:"sensitivityTags"`
	RiskNote        string   `json:"riskNote"`
}

func (s *Service) AddSegment(caseID string, in AddSegmentInput) (CommandResult, error) {
	id, err := requiredID(in.ID, "seg", s.newID)
	if err != nil {
		return CommandResult{}, err
	}
	return s.execute(caseID, "segment.add", in.CommandMeta, func(c *casefile.Case, now time.Time) error {
		return c.AddSegment(casefile.SegmentInput{
			ID: id, Sequence: in.Sequence, StartMillis: in.StartMillis, EndMillis: in.EndMillis,
			OriginalText: in.OriginalText, SpeakerLabel: in.SpeakerLabel,
			SensitivityTags: in.SensitivityTags, RiskNote: in.RiskNote,
		}, in.Actor, now)
	})
}

type BatchSegmentItem struct {
	ID              string   `json:"id"`
	Sequence        int      `json:"sequence"`
	StartMillis     int64    `json:"startMillis"`
	EndMillis       int64    `json:"endMillis"`
	OriginalText    string   `json:"originalText"`
	SpeakerLabel    string   `json:"speakerLabel"`
	SensitivityTags []string `json:"sensitivityTags"`
	RiskNote        string   `json:"riskNote"`
}

type AddSegmentsInput struct {
	CommandMeta
	Segments []BatchSegmentItem `json:"segments"`
}

func (s *Service) AddSegments(caseID string, in AddSegmentsInput) (CommandResult, error) {
	segments := make([]casefile.SegmentInput, len(in.Segments))
	for i, item := range in.Segments {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			var err error
			id, err = s.newID("seg")
			if err != nil {
				return CommandResult{}, fmt.Errorf("生成第 %d 行片段标识: %w", i+1, err)
			}
		}
		segments[i] = casefile.SegmentInput{
			ID: id, Sequence: item.Sequence, StartMillis: item.StartMillis, EndMillis: item.EndMillis,
			OriginalText: item.OriginalText, SpeakerLabel: item.SpeakerLabel,
			SensitivityTags: item.SensitivityTags, RiskNote: item.RiskNote,
		}
	}
	return s.execute(caseID, "segments.batch.add", in.CommandMeta, func(c *casefile.Case, now time.Time) error {
		return c.AddSegments(segments, in.Actor, now)
	})
}

type ReviseSegmentInput struct {
	CommandMeta
	ID              string   `json:"id"`
	Sequence        int      `json:"sequence"`
	StartMillis     int64    `json:"startMillis"`
	EndMillis       int64    `json:"endMillis"`
	OriginalText    string   `json:"originalText"`
	SpeakerLabel    string   `json:"speakerLabel"`
	SensitivityTags []string `json:"sensitivityTags"`
	RiskNote        string   `json:"riskNote"`
}

func (s *Service) ReviseSegment(caseID string, in ReviseSegmentInput) (CommandResult, error) {
	return s.execute(caseID, "segment.revise", in.CommandMeta, func(c *casefile.Case, now time.Time) error {
		return c.ReviseReturnedSegment(casefile.SegmentInput{
			ID: in.ID, Sequence: in.Sequence, StartMillis: in.StartMillis, EndMillis: in.EndMillis,
			OriginalText: in.OriginalText, SpeakerLabel: in.SpeakerLabel,
			SensitivityTags: in.SensitivityTags, RiskNote: in.RiskNote,
		}, in.Actor, now)
	})
}

func (s *Service) Freeze(caseID string, meta CommandMeta) (CommandResult, error) {
	return s.execute(caseID, "case.freeze", meta, func(c *casefile.Case, now time.Time) error {
		return c.Freeze(meta.Actor, now)
	})
}
