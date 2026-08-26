package release

import (
	"time"

	"oral-history-release-desk/internal/casefile"
)

func (s *Service) SubmitReview(caseID string, meta CommandMeta) (CommandResult, error) {
	return s.execute(caseID, "review.submit", meta, func(c *casefile.Case, now time.Time) error {
		return c.SubmitReview(meta.Actor, now)
	})
}

type ReviewDecisionInput struct {
	CommandMeta
	ID                 string                 `json:"id"`
	Outcome            casefile.ReviewOutcome `json:"outcome"`
	ReasonCode         string                 `json:"reasonCode"`
	Reason             string                 `json:"reason"`
	AffectedSegmentIDs []string               `json:"affectedSegmentIds"`
	Items              []casefile.ReviewItem  `json:"items"`
}

func (s *Service) ReviewDecision(caseID string, in ReviewDecisionInput) (CommandResult, error) {
	id, err := requiredID(in.ID, "review", s.newID)
	if err != nil {
		return CommandResult{}, err
	}
	return s.execute(caseID, "review.decision", in.CommandMeta, func(c *casefile.Case, now time.Time) error {
		return c.Review(casefile.ReviewInput{
			ID: id, Reviewer: in.Actor, Outcome: in.Outcome, ReasonCode: in.ReasonCode,
			Reason: in.Reason, AffectedSegmentIDs: in.AffectedSegmentIDs, Items: in.Items, Now: now,
		})
	})
}
