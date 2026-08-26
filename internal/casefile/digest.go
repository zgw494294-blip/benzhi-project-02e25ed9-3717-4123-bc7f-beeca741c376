package casefile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type packageDigestMaterial struct {
	ID               string             `json:"id"`
	CaseID           string             `json:"caseId"`
	CaseVersion      int64              `json:"caseVersion"`
	NormalizedText   string             `json:"normalizedText"`
	DecisionSnapshot []DecisionSnapshot `json:"decisionSnapshot"`
	ReviewSnapshot   []ReviewRecord     `json:"reviewSnapshot"`
	VersionSummary   string             `json:"versionSummary"`
	ApprovedBy       string             `json:"approvedBy"`
	SealedAt         string             `json:"sealedAt"`
}

func PackageDigest(pkg ReleasePackage) (string, error) {
	data, err := PackageCanonicalBytes(pkg)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func PackageCanonicalBytes(pkg ReleasePackage) ([]byte, error) {
	decisions := make([]DecisionSnapshot, len(pkg.DecisionSnapshot))
	copy(decisions, pkg.DecisionSnapshot)
	reviews := make([]ReviewRecord, len(pkg.ReviewSnapshot))
	copy(reviews, pkg.ReviewSnapshot)
	for i := range reviews {
		affected := make([]string, len(reviews[i].AffectedSegmentIDs))
		copy(affected, reviews[i].AffectedSegmentIDs)
		reviews[i].AffectedSegmentIDs = affected
		items := make([]ReviewItem, len(reviews[i].Items))
		copy(items, reviews[i].Items)
		reviews[i].Items = items
	}
	material := packageDigestMaterial{
		ID:               pkg.ID,
		CaseID:           pkg.CaseID,
		CaseVersion:      pkg.CaseVersion,
		NormalizedText:   pkg.NormalizedText,
		DecisionSnapshot: decisions,
		ReviewSnapshot:   reviews,
		VersionSummary:   pkg.VersionSummary,
		ApprovedBy:       pkg.ApprovedBy,
		SealedAt:         pkg.SealedAt.UTC().Format("2006-01-02T15:04:05.000000000Z"),
	}
	data, err := json.Marshal(material)
	if err != nil {
		return nil, fmt.Errorf("编码发布包摘要材料: %w", err)
	}
	return data, nil
}

func VerifyPackageDigest(pkg ReleasePackage) (bool, string, error) {
	calculated, err := PackageDigest(pkg)
	if err != nil {
		return false, "", err
	}
	return calculated == pkg.Digest, calculated, nil
}
