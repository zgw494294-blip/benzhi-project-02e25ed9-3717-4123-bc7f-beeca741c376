package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"oral-history-release-desk/internal/casefile"
)

type Engine struct{}

type Rule struct {
	Code        string
	Description string
	Evaluate    func(*casefile.Case, casefile.TranscriptSegment) *casefile.PolicyFinding
}

func New() *Engine {
	return &Engine{}
}

func stableID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:8])
}

func containsFold(values []string, terms ...string) bool {
	for _, value := range values {
		lower := strings.ToLower(value)
		for _, term := range terms {
			if strings.Contains(lower, strings.ToLower(term)) {
				return true
			}
		}
	}
	return false
}

func finding(c *casefile.Case, segment casefile.TranscriptSegment, runID, rule string, severity casefile.Severity, explanation string) casefile.PolicyFinding {
	status := casefile.ResolutionResolved
	if severity == casefile.SeverityBlock {
		status = casefile.ResolutionOpen
	}
	return casefile.PolicyFinding{
		ID:               stableID(c.ID, segment.ID, fmt.Sprint(segment.Revision), rule),
		CaseID:           c.ID,
		SegmentID:        segment.ID,
		SegmentRevision:  segment.Revision,
		RuleCode:         rule,
		Severity:         severity,
		Explanation:      explanation,
		ResolutionStatus: status,
		RunID:            runID,
	}
}
