package policy

import (
	"fmt"
	"sort"
	"strings"

	"oral-history-release-desk/internal/casefile"
)

func (e *Engine) Preview(c *casefile.Case) (casefile.Preview, error) {
	if c == nil {
		return casefile.Preview{}, fmt.Errorf("案件不能为空")
	}
	if c.HasOpenBlocks() {
		return casefile.Preview{}, fmt.Errorf("仍有阻断项未闭合")
	}
	segments := append([]casefile.TranscriptSegment(nil), c.Segments...)
	sort.SliceStable(segments, func(i, j int) bool { return segments[i].Sequence < segments[j].Sequence })
	preview := casefile.Preview{Segments: make([]casefile.PreviewSegment, 0, len(segments))}
	lines := make([]string, 0, len(segments))
	for _, segment := range segments {
		after, err := e.applySegmentDecisions(c, segment)
		if err != nil {
			return casefile.Preview{}, err
		}
		after = normalizeText(after)
		preview.Segments = append(preview.Segments, casefile.PreviewSegment{
			SegmentID: segment.ID,
			Sequence:  segment.Sequence,
			Before:    segment.OriginalText,
			After:     after,
		})
		lines = append(lines, fmt.Sprintf("[%04d] %s：%s", segment.Sequence, normalizeLabel(segment.SpeakerLabel), after))
	}
	preview.Text = strings.Join(lines, "\n")
	return preview, nil
}

func (e *Engine) applySegmentDecisions(c *casefile.Case, segment casefile.TranscriptSegment) (string, error) {
	findings := make([]casefile.PolicyFinding, 0)
	for _, item := range c.Findings {
		if item.SegmentID == segment.ID && item.Severity == casefile.SeverityBlock && item.ResolutionStatus != casefile.ResolutionObsolete {
			findings = append(findings, item)
		}
	}
	sort.SliceStable(findings, func(i, j int) bool { return findings[i].RuleCode < findings[j].RuleCode })
	result := segment.OriginalText
	for _, item := range findings {
		decision := c.DecisionFor(item.ID)
		if decision == nil {
			return "", fmt.Errorf("阻断项 %s 缺少处置决定", item.ID)
		}
		switch decision.Action {
		case casefile.ActionKeep:
		case casefile.ActionReplace:
			result = decision.ReplacementText
		case casefile.ActionRedact:
			result = "〔内容已遮蔽〕"
		default:
			return "", fmt.Errorf("阻断项 %s 的处置方式无效", item.ID)
		}
	}
	return result, nil
}

func (e *Engine) PreflightDecisionBatch(c *casefile.Case, decisions []casefile.RedactionDecision) ([]casefile.PreviewSegment, error) {
	bySegment := make(map[string][]casefile.RedactionDecision)
	selected := make(map[string]bool, len(decisions))
	for _, decision := range decisions {
		finding := c.Finding(decision.FindingID)
		if finding != nil {
			bySegment[finding.SegmentID] = append(bySegment[finding.SegmentID], decision)
			selected[decision.FindingID] = true
		}
	}
	// 将受影响片段上的既有有效决定一并纳入，避免分批提交绕过同片段冲突预检。
	for _, decision := range c.Decisions {
		if selected[decision.FindingID] {
			continue
		}
		finding := c.Finding(decision.FindingID)
		if finding == nil || finding.Severity != casefile.SeverityBlock || finding.ResolutionStatus == casefile.ResolutionObsolete {
			continue
		}
		if _, affected := bySegment[finding.SegmentID]; affected {
			bySegment[finding.SegmentID] = append(bySegment[finding.SegmentID], decision)
		}
	}
	segmentIDs := make([]string, 0, len(bySegment))
	for segmentID := range bySegment {
		segmentIDs = append(segmentIDs, segmentID)
	}
	sort.Strings(segmentIDs)
	previews := make([]casefile.PreviewSegment, 0, len(segmentIDs))
	conflicts := make([]casefile.DecisionConflict, 0)
	for _, segmentID := range segmentIDs {
		segment := c.Segment(segmentID)
		if segment == nil {
			continue
		}
		items := bySegment[segmentID]
		sort.SliceStable(items, func(i, j int) bool { return items[i].FindingID < items[j].FindingID })
		candidates := make(map[string]string, len(items))
		unique := make(map[string]bool)
		findingIDs := make([]string, 0, len(items))
		for _, decision := range items {
			candidate := normalizeText(segment.OriginalText)
			switch decision.Action {
			case casefile.ActionReplace:
				candidate = normalizeText(decision.ReplacementText)
			case casefile.ActionRedact:
				candidate = "〔内容已遮蔽〕"
			}
			candidates[decision.FindingID] = candidate
			unique[candidate] = true
			findingIDs = append(findingIDs, decision.FindingID)
		}
		if len(unique) > 1 {
			conflicts = append(conflicts, casefile.DecisionConflict{
				SegmentID: segmentID, FindingIDs: findingIDs, CandidateResult: candidates,
			})
			continue
		}
		after := normalizeText(segment.OriginalText)
		for candidate := range unique {
			after = candidate
		}
		previews = append(previews, casefile.PreviewSegment{
			SegmentID: segment.ID, Sequence: segment.Sequence, Before: segment.OriginalText, After: after,
		})
	}
	if len(conflicts) > 0 {
		return previews, casefile.DecisionConflictError{Conflicts: conflicts}
	}
	sort.SliceStable(previews, func(i, j int) bool { return previews[i].Sequence < previews[j].Sequence })
	return previews, nil
}

func normalizeText(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func normalizeLabel(value string) string {
	value = normalizeText(value)
	if value == "" {
		return "未标记人物"
	}
	return value
}
