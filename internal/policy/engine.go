package policy

import (
	"fmt"
	"sort"

	"oral-history-release-desk/internal/casefile"
)

func (e *Engine) Run(c *casefile.Case, runID string) ([]casefile.PolicyFinding, error) {
	if c == nil {
		return nil, fmt.Errorf("案件不能为空")
	}
	if runID == "" {
		return nil, fmt.Errorf("校核运行 ID 不能为空")
	}
	segments := append([]casefile.TranscriptSegment(nil), c.Segments...)
	sort.SliceStable(segments, func(i, j int) bool { return segments[i].Sequence < segments[j].Sequence })
	result := make([]casefile.PolicyFinding, 0)
	for _, segment := range segments {
		result = append(result, e.evaluateSegment(c, segment, runID)...)
	}
	sortFindings(result)
	return result, nil
}

func (e *Engine) RunTargeted(c *casefile.Case, runID string) ([]casefile.PolicyFinding, error) {
	if c == nil {
		return nil, fmt.Errorf("案件不能为空")
	}
	result := make([]casefile.PolicyFinding, 0)
	foundTarget := false
	for _, segment := range c.Segments {
		if !segment.NeedsRecheck {
			continue
		}
		foundTarget = true
		result = append(result, e.evaluateSegment(c, segment, runID)...)
	}
	if !foundTarget {
		return nil, fmt.Errorf("没有待定向校核的片段")
	}
	sortFindings(result)
	return result, nil
}

func sortFindings(findings []casefile.PolicyFinding) {
	severityRank := map[casefile.Severity]int{
		casefile.SeverityBlock: 0, casefile.SeverityWarning: 1, casefile.SeverityPass: 2,
	}
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].SegmentID != findings[j].SegmentID {
			return findings[i].SegmentID < findings[j].SegmentID
		}
		if severityRank[findings[i].Severity] != severityRank[findings[j].Severity] {
			return severityRank[findings[i].Severity] < severityRank[findings[j].Severity]
		}
		return findings[i].RuleCode < findings[j].RuleCode
	})
}
