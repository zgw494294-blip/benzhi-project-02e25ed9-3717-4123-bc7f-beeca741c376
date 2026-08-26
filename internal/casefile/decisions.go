package casefile

import (
	"fmt"
	"strings"
	"time"
)

type DecisionInput struct {
	ID              string
	FindingID       string
	Action          DecisionAction
	ReplacementText string
	Rationale       string
	DecidedBy       string
	Now             time.Time
}

func (c *Case) Decide(in DecisionInput) error {
	decisions, err := c.ValidateDecisionBatch([]DecisionInput{in})
	if err != nil {
		return err
	}
	c.ApplyDecisionBatch(decisions, in.DecidedBy, in.Now)
	return nil
}

func (c *Case) ValidateDecisionBatch(inputs []DecisionInput) ([]RedactionDecision, error) {
	if c.Status != StatusRemediation && c.Status != StatusReturned {
		return nil, StateError{Status: c.Status, Operation: "批量记录阻断处置"}
	}
	if len(inputs) == 0 {
		return nil, fieldError("decisions", "至少提交一项阻断处置")
	}
	errorsByField := make(map[string]string)
	seenFindings := make(map[string]int)
	result := make([]RedactionDecision, len(inputs))
	for i, in := range inputs {
		prefix := fmt.Sprintf("decisions[%d].", i)
		findingID := strings.TrimSpace(in.FindingID)
		finding := c.Finding(findingID)
		if finding == nil || finding.Severity != SeverityBlock || finding.ResolutionStatus == ResolutionObsolete {
			errorsByField[prefix+"findingId"] = "可处置的未过期阻断项不存在"
		}
		if previous, exists := seenFindings[findingID]; findingID != "" && exists {
			errorsByField[prefix+"findingId"] = fmt.Sprintf("与第 %d 项阻断处置重复", previous+1)
			errorsByField[fmt.Sprintf("decisions[%d].findingId", previous)] = fmt.Sprintf("与第 %d 项阻断处置重复", i+1)
		} else {
			seenFindings[findingID] = i
		}
		if strings.TrimSpace(in.ID) == "" {
			errorsByField[prefix+"id"] = "处置决定 ID 不能为空"
		}
		if in.Action != ActionKeep && in.Action != ActionReplace && in.Action != ActionRedact {
			errorsByField[prefix+"action"] = "处置方式必须为 keep、replace 或 redact"
		}
		replacement := strings.Join(strings.Fields(strings.TrimSpace(in.ReplacementText)), " ")
		if in.Action == ActionReplace && replacement == "" {
			errorsByField[prefix+"replacementText"] = "替换处置必须提供替代文本"
		}
		if strings.TrimSpace(in.Rationale) == "" {
			errorsByField[prefix+"rationale"] = "处置理由不能为空"
		}
		if strings.TrimSpace(in.DecidedBy) == "" {
			errorsByField[prefix+"decidedBy"] = "处置人不能为空"
		}
		result[i] = RedactionDecision{
			ID: strings.TrimSpace(in.ID), FindingID: findingID, Action: in.Action,
			ReplacementText: replacement, Rationale: strings.TrimSpace(in.Rationale),
			DecidedBy: strings.TrimSpace(in.DecidedBy), DecidedAt: in.Now.UTC(),
		}
	}
	if len(errorsByField) > 0 {
		return nil, MultiValidationError{Fields: errorsByField}
	}
	return result, nil
}

func (c *Case) ApplyDecisionBatch(decisions []RedactionDecision, actor string, now time.Time) {
	for _, decision := range decisions {
		replaced := false
		for i := range c.Decisions {
			if c.Decisions[i].FindingID == decision.FindingID {
				c.Decisions[i] = decision
				replaced = true
				break
			}
		}
		if !replaced {
			c.Decisions = append(c.Decisions, decision)
		}
		c.Finding(decision.FindingID).ResolutionStatus = ResolutionResolved
	}
	c.Touch(now)
	c.AppendEvent("findings.batch_resolved", c.Status, c.Status, actor,
		fmt.Sprintf("批量闭合 %d 个阻断项，剩余 %d 个", len(decisions), len(c.OpenBlocks())), now)
}

func (c *Case) Finding(id string) *PolicyFinding {
	for i := range c.Findings {
		if c.Findings[i].ID == id {
			return &c.Findings[i]
		}
	}
	return nil
}

func (c *Case) DecisionFor(findingID string) *RedactionDecision {
	for i := range c.Decisions {
		if c.Decisions[i].FindingID == findingID {
			return &c.Decisions[i]
		}
	}
	return nil
}
