package policy

import (
	"sort"
	"strings"

	"oral-history-release-desk/internal/casefile"
)

type RiskFilter struct {
	Severity  casefile.Severity
	RuleCode  string
	SegmentID string
	Changed   *bool
}

type RiskCounts struct {
	Pass       int `json:"pass"`
	Warning    int `json:"warning"`
	Block      int `json:"block"`
	OpenBlocks int `json:"openBlocks"`
}

type RiskGroup struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

type FindingView struct {
	FindingID       string                    `json:"findingId"`
	RunID           string                    `json:"runId"`
	SegmentID       string                    `json:"segmentId"`
	SegmentRevision int64                     `json:"segmentRevision"`
	Sequence        int                       `json:"sequence"`
	RuleCode        string                    `json:"ruleCode"`
	Severity        casefile.Severity         `json:"severity"`
	Resolution      casefile.ResolutionStatus `json:"resolutionStatus"`
	Explanation     string                    `json:"explanation"`
	Change          string                    `json:"change"`
}

type FindingConclusion struct {
	FindingID   string            `json:"findingId"`
	RunID       string            `json:"runId"`
	Revision    int64             `json:"segmentRevision"`
	Severity    casefile.Severity `json:"severity"`
	Explanation string            `json:"explanation"`
}

type FindingDifference struct {
	SegmentID string             `json:"segmentId"`
	Sequence  int                `json:"sequence"`
	RuleCode  string             `json:"ruleCode"`
	Status    string             `json:"status"`
	Changed   bool               `json:"changed"`
	Previous  *FindingConclusion `json:"previous"`
	Current   *FindingConclusion `json:"current"`
}

type RiskSummary struct {
	RunID             string              `json:"runId"`
	Counts            RiskCounts          `json:"counts"`
	ByRuleCode        []RiskGroup         `json:"byRuleCode"`
	BySegment         []RiskGroup         `json:"bySegment"`
	BySensitivity     []RiskGroup         `json:"bySensitivity"`
	Findings          []FindingView       `json:"findings"`
	Differences       []FindingDifference `json:"differences"`
	MatchedFindingNum int                 `json:"matchedFindingCount"`
}

func BuildRiskSummary(c *casefile.Case, filter RiskFilter) RiskSummary {
	active := make([]casefile.PolicyFinding, 0)
	obsolete := make([]casefile.PolicyFinding, 0)
	for _, finding := range c.Findings {
		if finding.ResolutionStatus == casefile.ResolutionObsolete {
			obsolete = append(obsolete, finding)
		} else {
			active = append(active, finding)
		}
	}
	differences := buildDifferences(c, active, obsolete)
	changeByPair := make(map[string]string, len(differences))
	for _, item := range differences {
		if item.Current != nil {
			changeByPair[pairKey(item.SegmentID, item.RuleCode)] = item.Status
		}
	}
	sequence := make(map[string]int, len(c.Segments))
	tags := make(map[string][]string, len(c.Segments))
	for _, segment := range c.Segments {
		sequence[segment.ID] = segment.Sequence
		tags[segment.ID] = segment.SensitivityTags
	}
	result := RiskSummary{
		RunID: c.LastRunID, ByRuleCode: []RiskGroup{}, BySegment: []RiskGroup{},
		BySensitivity: []RiskGroup{}, Findings: []FindingView{}, Differences: []FindingDifference{},
	}
	ruleCounts := make(map[string]int)
	segmentCounts := make(map[string]int)
	sensitivityCounts := make(map[string]int)
	for _, finding := range active {
		change := changeByPair[pairKey(finding.SegmentID, finding.RuleCode)]
		if change == "" {
			change = "unchanged"
		}
		changed := change != "unchanged"
		if !matchesFilter(finding.Severity, finding.RuleCode, finding.SegmentID, changed, filter) {
			continue
		}
		switch finding.Severity {
		case casefile.SeverityPass:
			result.Counts.Pass++
		case casefile.SeverityWarning:
			result.Counts.Warning++
		case casefile.SeverityBlock:
			result.Counts.Block++
			if finding.ResolutionStatus == casefile.ResolutionOpen {
				result.Counts.OpenBlocks++
			}
		}
		ruleCounts[finding.RuleCode]++
		segmentCounts[finding.SegmentID]++
		segmentTags := tags[finding.SegmentID]
		if len(segmentTags) == 0 {
			sensitivityCounts["未标注"]++
		} else {
			for _, tag := range segmentTags {
				sensitivityCounts[tag]++
			}
		}
		result.Findings = append(result.Findings, FindingView{
			FindingID: finding.ID, RunID: finding.RunID, SegmentID: finding.SegmentID,
			SegmentRevision: finding.SegmentRevision, Sequence: sequence[finding.SegmentID],
			RuleCode: finding.RuleCode, Severity: finding.Severity, Resolution: finding.ResolutionStatus,
			Explanation: finding.Explanation, Change: change,
		})
	}
	for _, difference := range differences {
		severity := casefile.Severity("")
		if difference.Current != nil {
			severity = difference.Current.Severity
		} else if difference.Previous != nil {
			severity = difference.Previous.Severity
		}
		if matchesFilter(severity, difference.RuleCode, difference.SegmentID, difference.Changed, filter) {
			result.Differences = append(result.Differences, difference)
		}
	}
	result.ByRuleCode = sortedGroups(ruleCounts)
	result.BySegment = sortedGroups(segmentCounts)
	result.BySensitivity = sortedGroups(sensitivityCounts)
	result.MatchedFindingNum = len(result.Findings)
	sort.SliceStable(result.Findings, func(i, j int) bool {
		if result.Findings[i].Sequence != result.Findings[j].Sequence {
			return result.Findings[i].Sequence < result.Findings[j].Sequence
		}
		if result.Findings[i].RuleCode != result.Findings[j].RuleCode {
			return result.Findings[i].RuleCode < result.Findings[j].RuleCode
		}
		return result.Findings[i].FindingID < result.Findings[j].FindingID
	})
	return result
}

func buildDifferences(c *casefile.Case, active, obsolete []casefile.PolicyFinding) []FindingDifference {
	activeByPair := make(map[string]casefile.PolicyFinding)
	oldByPair := make(map[string]casefile.PolicyFinding)
	rerunSegments := make(map[string]bool)
	sequence := make(map[string]int)
	for _, segment := range c.Segments {
		sequence[segment.ID] = segment.Sequence
	}
	for _, finding := range active {
		activeByPair[pairKey(finding.SegmentID, finding.RuleCode)] = finding
		if finding.RunID == c.LastRunID {
			rerunSegments[finding.SegmentID] = true
		}
	}
	for _, finding := range obsolete {
		key := pairKey(finding.SegmentID, finding.RuleCode)
		previous, exists := oldByPair[key]
		if !exists || finding.SegmentRevision > previous.SegmentRevision {
			oldByPair[key] = finding
		}
		if finding.RunID != c.LastRunID {
			rerunSegments[finding.SegmentID] = true
		}
	}
	keys := make(map[string]bool)
	for key := range activeByPair {
		keys[key] = true
	}
	for key := range oldByPair {
		keys[key] = true
	}
	result := make([]FindingDifference, 0, len(keys))
	for key := range keys {
		current, hasCurrent := activeByPair[key]
		previous, hasPrevious := oldByPair[key]
		segmentID, ruleCode := splitPair(key)
		status := "unchanged"
		if rerunSegments[segmentID] {
			switch {
			case !hasPrevious && hasCurrent:
				status = "added"
			case hasPrevious && !hasCurrent:
				status = "removed"
			case hasPrevious && hasCurrent && previous.Severity != current.Severity:
				status = "severity_changed"
			case hasPrevious && hasCurrent && previous.Explanation != current.Explanation:
				status = "evidence_changed"
			}
		}
		item := FindingDifference{SegmentID: segmentID, Sequence: sequence[segmentID], RuleCode: ruleCode, Status: status, Changed: status != "unchanged"}
		if hasPrevious {
			item.Previous = conclusion(previous)
		}
		if hasCurrent {
			item.Current = conclusion(current)
		}
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Sequence != result[j].Sequence {
			return result[i].Sequence < result[j].Sequence
		}
		if result[i].RuleCode != result[j].RuleCode {
			return result[i].RuleCode < result[j].RuleCode
		}
		return result[i].Status < result[j].Status
	})
	return result
}

func conclusion(f casefile.PolicyFinding) *FindingConclusion {
	return &FindingConclusion{FindingID: f.ID, RunID: f.RunID, Revision: f.SegmentRevision, Severity: f.Severity, Explanation: f.Explanation}
}

func matchesFilter(severity casefile.Severity, ruleCode, segmentID string, changed bool, filter RiskFilter) bool {
	return (filter.Severity == "" || severity == filter.Severity) &&
		(filter.RuleCode == "" || ruleCode == filter.RuleCode) &&
		(filter.SegmentID == "" || segmentID == filter.SegmentID) &&
		(filter.Changed == nil || changed == *filter.Changed)
}

func sortedGroups(counts map[string]int) []RiskGroup {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]RiskGroup, 0, len(keys))
	for _, key := range keys {
		result = append(result, RiskGroup{Key: key, Count: counts[key]})
	}
	return result
}

func pairKey(segmentID, ruleCode string) string { return segmentID + "\x00" + ruleCode }

func splitPair(key string) (string, string) {
	parts := strings.SplitN(key, "\x00", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}
