package casefile

import "time"

type Status string

const (
	StatusDraft           Status = "draft"
	StatusPendingCheck    Status = "pending_check"
	StatusRemediation     Status = "remediation"
	StatusPendingReview   Status = "pending_review"
	StatusReturned        Status = "returned"
	StatusPendingApproval Status = "pending_approval"
	StatusSealed          Status = "sealed"
)

type Severity string

const (
	SeverityPass    Severity = "pass"
	SeverityWarning Severity = "warning"
	SeverityBlock   Severity = "block"
)

type ResolutionStatus string

const (
	ResolutionOpen     ResolutionStatus = "open"
	ResolutionResolved ResolutionStatus = "resolved"
	ResolutionObsolete ResolutionStatus = "obsolete"
)

type DecisionAction string

const (
	ActionKeep    DecisionAction = "keep"
	ActionReplace DecisionAction = "replace"
	ActionRedact  DecisionAction = "redact"
)

type ReviewOutcome string

const (
	ReviewApproved ReviewOutcome = "approved"
	ReviewReturned ReviewOutcome = "returned"
)

type Case struct {
	ID               string              `json:"id"`
	Title            string              `json:"title"`
	InterviewDate    string              `json:"interviewDate"`
	IntendedUse      string              `json:"intendedUse"`
	ConsentScope     []string            `json:"consentScope"`
	RestrictionTerms []string            `json:"restrictionTerms"`
	Status           Status              `json:"status"`
	Version          int64               `json:"version"`
	CreatedAt        time.Time           `json:"createdAt"`
	UpdatedAt        time.Time           `json:"updatedAt"`
	Segments         []TranscriptSegment `json:"segments"`
	Findings         []PolicyFinding     `json:"findings"`
	Decisions        []RedactionDecision `json:"decisions"`
	Reviews          []ReviewRecord      `json:"reviews"`
	Timeline         []TimelineEvent     `json:"timeline"`
	Preview          *Preview            `json:"preview,omitempty"`
	Package          *ReleasePackage     `json:"releasePackage,omitempty"`
	LastRunID        string              `json:"lastRunId,omitempty"`
}

type TranscriptSegment struct {
	ID              string   `json:"id"`
	CaseID          string   `json:"caseId"`
	Sequence        int      `json:"sequence"`
	StartMillis     int64    `json:"startMillis"`
	EndMillis       int64    `json:"endMillis"`
	OriginalText    string   `json:"originalText"`
	SpeakerLabel    string   `json:"speakerLabel"`
	SensitivityTags []string `json:"sensitivityTags"`
	RiskNote        string   `json:"riskNote"`
	Revision        int64    `json:"revision"`
	NeedsRecheck    bool     `json:"needsRecheck"`
}

type PolicyFinding struct {
	ID               string           `json:"id"`
	CaseID           string           `json:"caseId"`
	SegmentID        string           `json:"segmentId,omitempty"`
	SegmentRevision  int64            `json:"segmentRevision,omitempty"`
	RuleCode         string           `json:"ruleCode"`
	Severity         Severity         `json:"severity"`
	Explanation      string           `json:"explanation"`
	ResolutionStatus ResolutionStatus `json:"resolutionStatus"`
	RunID            string           `json:"runId"`
}

type RedactionDecision struct {
	ID              string         `json:"id"`
	FindingID       string         `json:"findingId"`
	Action          DecisionAction `json:"action"`
	ReplacementText string         `json:"replacementText,omitempty"`
	Rationale       string         `json:"rationale"`
	DecidedBy       string         `json:"decidedBy"`
	DecidedAt       time.Time      `json:"decidedAt"`
}

type ReviewItem struct {
	FindingID string `json:"findingId"`
	Confirmed bool   `json:"confirmed"`
	Note      string `json:"note,omitempty"`
}

type ReviewRecord struct {
	ID                 string        `json:"id"`
	Reviewer           string        `json:"reviewer"`
	Outcome            ReviewOutcome `json:"outcome"`
	ReasonCode         string        `json:"reasonCode,omitempty"`
	Reason             string        `json:"reason,omitempty"`
	AffectedSegmentIDs []string      `json:"affectedSegmentIds,omitempty"`
	Items              []ReviewItem  `json:"items"`
	CaseVersion        int64         `json:"caseVersion"`
	CreatedAt          time.Time     `json:"createdAt"`
}

type PreviewSegment struct {
	SegmentID string `json:"segmentId"`
	Sequence  int    `json:"sequence"`
	Before    string `json:"before"`
	After     string `json:"after"`
}

type Preview struct {
	CaseVersion int64            `json:"caseVersion"`
	GeneratedAt time.Time        `json:"generatedAt"`
	Segments    []PreviewSegment `json:"segments"`
	Text        string           `json:"text"`
}

type DecisionSnapshot struct {
	FindingID  string         `json:"findingId"`
	RuleCode   string         `json:"ruleCode"`
	Action     DecisionAction `json:"action"`
	ResultText string         `json:"resultText"`
	Rationale  string         `json:"rationale"`
}

type ReleasePackage struct {
	ID               string             `json:"id"`
	CaseID           string             `json:"caseId"`
	CaseVersion      int64              `json:"caseVersion"`
	NormalizedText   string             `json:"normalizedText"`
	DecisionSnapshot []DecisionSnapshot `json:"decisionSnapshot"`
	ReviewSnapshot   []ReviewRecord     `json:"reviewSnapshot"`
	VersionSummary   string             `json:"versionSummary"`
	ApprovedBy       string             `json:"approvedBy"`
	SealedAt         time.Time          `json:"sealedAt"`
	Digest           string             `json:"digest"`
}

type TimelineEvent struct {
	Sequence    int64     `json:"sequence"`
	Type        string    `json:"type"`
	FromStatus  Status    `json:"fromStatus,omitempty"`
	ToStatus    Status    `json:"toStatus,omitempty"`
	Actor       string    `json:"actor"`
	Reason      string    `json:"reason,omitempty"`
	CaseVersion int64     `json:"caseVersion"`
	At          time.Time `json:"at"`
}
