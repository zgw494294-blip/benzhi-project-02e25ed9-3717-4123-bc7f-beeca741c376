package release

import (
	"strings"
	"time"

	"oral-history-release-desk/internal/casefile"
	"oral-history-release-desk/internal/policy"
)

type RiskSummaryFilter struct {
	Severity  string
	RuleCode  string
	SegmentID string
	Changed   *bool
}

type riskSummaryCacheEntry struct {
	version int64
	summary policy.RiskSummary
}

func (s *Service) RiskSummary(caseID string, filter RiskSummaryFilter) (policy.RiskSummary, error) {
	severity := casefile.Severity(strings.TrimSpace(filter.Severity))
	if severity != "" && severity != casefile.SeverityPass && severity != casefile.SeverityWarning && severity != casefile.SeverityBlock {
		return policy.RiskSummary{}, casefile.ValidationError{Field: "severity", Message: "severity 必须为 pass、warning 或 block"}
	}
	ruleCode := strings.TrimSpace(filter.RuleCode)
	if ruleCode != "" && !validFilterToken(ruleCode) {
		return policy.RiskSummary{}, casefile.ValidationError{Field: "ruleCode", Message: "ruleCode 筛选格式无效"}
	}
	segmentID := strings.TrimSpace(filter.SegmentID)
	if segmentID != "" && (len(segmentID) > 128 || strings.ContainsAny(segmentID, "/\\\x00")) {
		return policy.RiskSummary{}, casefile.ValidationError{Field: "segmentId", Message: "segmentId 筛选格式无效"}
	}
	c, err := s.store.Get(caseID)
	if err != nil {
		return policy.RiskSummary{}, err
	}
	base := s.cachedRiskSummary(c)
	if severity == "" && ruleCode == "" && segmentID == "" && filter.Changed == nil {
		return base, nil
	}
	filtered := policy.BuildRiskSummary(c, policy.RiskFilter{Severity: severity, RuleCode: ruleCode, SegmentID: segmentID, Changed: filter.Changed})
	return filtered, nil
}

func (s *Service) cachedRiskSummary(c *casefile.Case) policy.RiskSummary {
	s.riskMu.Lock()
	defer s.riskMu.Unlock()
	if cached, ok := s.riskSummaries[c.ID]; ok && cached.version == c.Version {
		return cached.summary
	}
	summary := policy.BuildRiskSummary(c, policy.RiskFilter{})
	s.riskSummaries[c.ID] = riskSummaryCacheEntry{version: c.Version, summary: summary}
	return summary
}

func validFilterToken(value string) bool {
	for _, char := range value {
		if (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '_' && char != '-' {
			return false
		}
	}
	return len(value) <= 128
}

func (s *Service) RunCheck(caseID string, meta CommandMeta) (CommandResult, error) {
	runID, err := s.newID("run")
	if err != nil {
		return CommandResult{}, err
	}
	return s.execute(caseID, "policy.check", meta, func(c *casefile.Case, now time.Time) error {
		findings, err := s.policy.Run(c, runID)
		if err != nil {
			return err
		}
		return c.ApplyFindings(runID, findings, false, meta.Actor, now)
	})
}

func (s *Service) RunTargetedCheck(caseID string, meta CommandMeta) (CommandResult, error) {
	runID, err := s.newID("run")
	if err != nil {
		return CommandResult{}, err
	}
	return s.execute(caseID, "policy.recheck", meta, func(c *casefile.Case, now time.Time) error {
		findings, err := s.policy.RunTargeted(c, runID)
		if err != nil {
			return err
		}
		return c.ApplyFindings(runID, findings, true, meta.Actor, now)
	})
}

type DecideFindingInput struct {
	CommandMeta
	ID              string                  `json:"id"`
	FindingID       string                  `json:"findingId"`
	Action          casefile.DecisionAction `json:"action"`
	ReplacementText string                  `json:"replacementText"`
	Rationale       string                  `json:"rationale"`
}

type BatchDecisionItem struct {
	ID              string                  `json:"id"`
	FindingID       string                  `json:"findingId"`
	Action          casefile.DecisionAction `json:"action"`
	ReplacementText string                  `json:"replacementText"`
	Rationale       string                  `json:"rationale"`
}

type DecideFindingsInput struct {
	CommandMeta
	Decisions []BatchDecisionItem `json:"decisions"`
}

func (s *Service) DecideFinding(caseID string, in DecideFindingInput) (CommandResult, error) {
	return s.decideFindings(caseID, "finding.decide", in.CommandMeta, []BatchDecisionItem{{
		ID: in.ID, FindingID: in.FindingID, Action: in.Action,
		ReplacementText: in.ReplacementText, Rationale: in.Rationale,
	}})
}

func (s *Service) DecideFindings(caseID string, in DecideFindingsInput) (CommandResult, error) {
	return s.decideFindings(caseID, "findings.batch.decide", in.CommandMeta, in.Decisions)
}

func (s *Service) decideFindings(caseID, operation string, meta CommandMeta, items []BatchDecisionItem) (CommandResult, error) {
	inputs := make([]casefile.DecisionInput, len(items))
	for i, item := range items {
		id, err := requiredID(item.ID, "decision", s.newID)
		if err != nil {
			return CommandResult{}, err
		}
		inputs[i] = casefile.DecisionInput{
			ID: id, FindingID: item.FindingID, Action: item.Action, ReplacementText: item.ReplacementText,
			Rationale: item.Rationale, DecidedBy: meta.Actor,
		}
	}
	return s.execute(caseID, operation, meta, func(c *casefile.Case, now time.Time) error {
		for i := range inputs {
			inputs[i].Now = now
		}
		decisions, err := c.ValidateDecisionBatch(inputs)
		if err != nil {
			return err
		}
		if _, err := s.policy.PreflightDecisionBatch(c, decisions); err != nil {
			return err
		}
		c.ApplyDecisionBatch(decisions, meta.Actor, now)
		return nil
	})
}

func (s *Service) GeneratePreview(caseID string, meta CommandMeta) (CommandResult, error) {
	return s.execute(caseID, "preview.generate", meta, func(c *casefile.Case, now time.Time) error {
		preview, err := s.policy.Preview(c)
		if err != nil {
			return err
		}
		return c.SetPreview(preview, meta.Actor, now)
	})
}
