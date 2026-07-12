package adminresource

import (
	"fmt"
	"strings"
)

type PolicyReviewResult struct {
	Review Record `json:"review"`
	Role   Record `json:"role"`
	Audit  Record `json:"audit,omitempty"`
}

type PolicyReviewActionResult struct {
	Review Record `json:"review"`
	Audit  Record `json:"audit,omitempty"`
}

type PolicyReviewExport struct {
	ExportedBy string                      `json:"exportedBy"`
	ExportedAt string                      `json:"exportedAt"`
	Watermark  PolicyReviewExportWatermark `json:"watermark"`
	Reviews    []Record                    `json:"reviews"`
	Audits     []Record                    `json:"audits"`
}

type PolicyReviewExportWatermark struct {
	Applied    bool   `json:"applied"`
	Product    string `json:"product"`
	ExportedBy string `json:"exportedBy"`
	ExportedAt string `json:"exportedAt"`
}

func (s *Store) RequestPolicyReview(reviewID string, requesterCode string, auditActorID string) (PolicyReviewActionResult, error) {
	return s.transitionPolicyReview(reviewID, policyReviewTransition{
		actorCode:    requesterCode,
		auditActorID: auditActorID,
		fromStatus:   "draft",
		toStatus:     "pending",
		auditSuffix:  "requested",
		auditAction:  "policy-review.request",
		auditName:    "Policy review requested",
		apply: func(review *Record, actorCode string, now string) {
			review.Values["requestedBy"] = strings.TrimSpace(actorCode)
			review.Values["submittedAt"] = now
		},
	})
}

func (s *Store) ApprovePolicyReview(reviewID string, reviewerCode string, auditActorID string) (PolicyReviewResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.prepareMutationLocked()
	if err != nil {
		return PolicyReviewResult{}, err
	}

	reviews, ok := s.resources["policy-reviews"]
	if !ok {
		return PolicyReviewResult{}, ErrUnknownResource
	}
	reviewIndex := recordIndexByID(reviews, reviewID)
	if reviewIndex < 0 {
		return PolicyReviewResult{}, ErrRecordNotFound
	}
	review := cloneRecord(reviews[reviewIndex])
	if strings.TrimSpace(review.Values["reviewStatus"]) != "pending" {
		return PolicyReviewResult{}, ValidationError{Field: "reviewStatus"}
	}

	roleCode := strings.TrimSpace(review.Values["roleCode"])
	if roleCode == "" {
		return PolicyReviewResult{}, ValidationError{Field: "roleCode"}
	}
	roles, ok := s.resources["roles"]
	if !ok {
		return PolicyReviewResult{}, ErrUnknownResource
	}
	roleIndex := recordIndexByCode(roles, roleCode)
	if roleIndex < 0 {
		return PolicyReviewResult{}, ErrRecordNotFound
	}

	audits, ok := s.resources["audit-logs"]
	if !ok {
		return PolicyReviewResult{}, ErrUnknownResource
	}

	now := s.now().UTC().Format("2006-01-02T15:04:05Z07:00")
	role := cloneRecord(roles[roleIndex])
	if role.Values == nil {
		role.Values = map[string]string{}
	}
	if err := applyPolicyReviewToRole(review, &role); err != nil {
		return PolicyReviewResult{}, err
	}
	role.UpdatedAt = now
	roles[roleIndex] = role
	s.resources["roles"] = roles

	updatedReview := cloneRecord(review)
	if updatedReview.Values == nil {
		updatedReview.Values = map[string]string{}
	}
	updatedReview.Values["reviewStatus"] = "approved"
	updatedReview.Values["reviewedBy"] = strings.TrimSpace(reviewerCode)
	updatedReview.Values["reviewedAt"] = now
	updatedReview.UpdatedAt = now
	reviews[reviewIndex] = updatedReview
	s.resources["policy-reviews"] = reviews

	audit, err := s.auditRecordLocked(AuditEvent{
		Actor: strings.TrimSpace(auditActorID), Action: "policy-review.approve", Resource: "roles",
		TargetID: role.ID, Result: "success", ReasonCode: "approved",
	}, s.nextID+1)
	if err != nil {
		s.restoreSnapshotLocked(previous)
		return PolicyReviewResult{}, err
	}
	s.nextID++
	audits = append(audits, audit)
	s.resources["audit-logs"] = audits

	if err := s.persistLocked(); err != nil {
		s.restoreSnapshotLocked(previous)
		return PolicyReviewResult{}, err
	}

	return PolicyReviewResult{Review: cloneRecord(updatedReview), Role: cloneRecord(role), Audit: cloneRecord(audit)}, nil
}

func (s *Store) RejectPolicyReview(reviewID string, reviewerCode string, auditActorID string, reason string) (PolicyReviewActionResult, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return PolicyReviewActionResult{}, ValidationError{Field: "reason"}
	}
	return s.transitionPolicyReview(reviewID, policyReviewTransition{
		actorCode:    reviewerCode,
		auditActorID: auditActorID,
		fromStatus:   "pending",
		toStatus:     "rejected",
		auditSuffix:  "rejected",
		auditAction:  "policy-review.reject",
		auditName:    "Policy review rejected",
		apply: func(review *Record, actorCode string, now string) {
			review.Values["reviewedBy"] = strings.TrimSpace(actorCode)
			review.Values["reviewedAt"] = now
			review.Values["rejectionReason"] = reason
		},
	})
}

func (s *Store) ExportPolicyReviews(actorCode string, auditActorID string, watermarkApplied bool) (PolicyReviewExport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.prepareMutationLocked()
	if err != nil {
		return PolicyReviewExport{}, err
	}

	reviews, ok := s.resources["policy-reviews"]
	if !ok {
		return PolicyReviewExport{}, ErrUnknownResource
	}
	audits, ok := s.resources["audit-logs"]
	if !ok {
		return PolicyReviewExport{}, ErrUnknownResource
	}

	projectedReviews := make([]Record, 0, len(reviews))
	for _, review := range reviews {
		projected, projectErr := s.projectRecordLocked("policy-reviews", review, ProjectionExport)
		if projectErr != nil {
			return PolicyReviewExport{}, projectErr
		}
		projectedReviews = append(projectedReviews, projected)
	}
	projectedAudits := make([]Record, 0, len(audits)+1)
	for _, existing := range policyReviewAuditRecords(audits) {
		projected, projectErr := s.projectRecordLocked("audit-logs", existing, ProjectionExport)
		if projectErr != nil {
			return PolicyReviewExport{}, projectErr
		}
		projectedAudits = append(projectedAudits, projected)
	}
	now := s.now().UTC().Format("2006-01-02T15:04:05Z07:00")
	audit, err := s.auditRecordLocked(AuditEvent{
		Actor: strings.TrimSpace(auditActorID), Action: "policy-review.export", Resource: "policy-reviews",
		TargetID: "policy-reviews", Result: "success", ReasonCode: fmt.Sprintf("watermarkApplied=%t", watermarkApplied),
	}, s.nextID+1)
	if err != nil {
		return PolicyReviewExport{}, err
	}
	projectedAudit, err := s.projectRecordLocked("audit-logs", audit, ProjectionExport)
	if err != nil {
		return PolicyReviewExport{}, err
	}
	s.nextID++
	audits = append(audits, audit)
	s.resources["audit-logs"] = audits
	if err := s.persistLocked(); err != nil {
		s.restoreSnapshotLocked(previous)
		return PolicyReviewExport{}, err
	}

	exportedBy := strings.TrimSpace(actorCode)
	productName := s.brandingConfigLocked().ProductName
	return PolicyReviewExport{
		ExportedBy: exportedBy,
		ExportedAt: now,
		Watermark: PolicyReviewExportWatermark{
			Applied: watermarkApplied, Product: productName, ExportedBy: exportedBy, ExportedAt: now,
		},
		Reviews: projectedReviews,
		Audits:  append(projectedAudits, projectedAudit),
	}, nil
}

type policyReviewTransition struct {
	actorCode    string
	auditActorID string
	fromStatus   string
	toStatus     string
	auditSuffix  string
	auditAction  string
	auditName    string
	apply        func(review *Record, actorCode string, now string)
}

func (s *Store) transitionPolicyReview(reviewID string, transition policyReviewTransition) (PolicyReviewActionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.prepareMutationLocked()
	if err != nil {
		return PolicyReviewActionResult{}, err
	}

	reviews, ok := s.resources["policy-reviews"]
	if !ok {
		return PolicyReviewActionResult{}, ErrUnknownResource
	}
	reviewIndex := recordIndexByID(reviews, reviewID)
	if reviewIndex < 0 {
		return PolicyReviewActionResult{}, ErrRecordNotFound
	}
	review := cloneRecord(reviews[reviewIndex])
	if strings.TrimSpace(review.Values["reviewStatus"]) != transition.fromStatus {
		return PolicyReviewActionResult{}, ValidationError{Field: "reviewStatus"}
	}
	audits, ok := s.resources["audit-logs"]
	if !ok {
		return PolicyReviewActionResult{}, ErrUnknownResource
	}

	now := s.now().UTC().Format("2006-01-02T15:04:05Z07:00")
	updatedReview := cloneRecord(review)
	if updatedReview.Values == nil {
		updatedReview.Values = map[string]string{}
	}
	updatedReview.Values["reviewStatus"] = transition.toStatus
	if transition.apply != nil {
		transition.apply(&updatedReview, transition.actorCode, now)
	}
	updatedReview.UpdatedAt = now
	reviews[reviewIndex] = updatedReview
	s.resources["policy-reviews"] = reviews

	audit, err := s.auditRecordLocked(AuditEvent{
		Actor: strings.TrimSpace(transition.auditActorID), Action: transition.auditAction, Resource: "policy-reviews",
		TargetID: review.ID, Result: "success", ReasonCode: transition.auditSuffix,
	}, s.nextID+1)
	if err != nil {
		s.restoreSnapshotLocked(previous)
		return PolicyReviewActionResult{}, err
	}
	s.nextID++
	audits = append(audits, audit)
	s.resources["audit-logs"] = audits
	if err := s.persistLocked(); err != nil {
		s.restoreSnapshotLocked(previous)
		return PolicyReviewActionResult{}, err
	}
	return PolicyReviewActionResult{Review: cloneRecord(updatedReview), Audit: cloneRecord(audit)}, nil
}

func applyPolicyReviewToRole(review Record, role *Record) error {
	if strings.TrimSpace(review.Values["requestedAction"]) != "update" {
		return ValidationError{Field: "requestedAction"}
	}
	switch strings.TrimSpace(review.Values["policyType"]) {
	case "role_permission":
		role.Values["permissions"] = strings.TrimSpace(review.Values["permissionCodes"])
	case "deny_permission":
		role.Values["denyPermissions"] = strings.TrimSpace(review.Values["permissionCodes"])
	case "data_scope":
		dataScope := strings.TrimSpace(review.Values["dataScope"])
		if dataScope == "" {
			return ValidationError{Field: "dataScope"}
		}
		role.Values["dataScope"] = dataScope
		role.Values["dataScopeOrgCodes"] = strings.TrimSpace(review.Values["dataScopeOrgCodes"])
		role.Values["dataScopeAreaCodes"] = strings.TrimSpace(review.Values["dataScopeAreaCodes"])
	default:
		return ValidationError{Field: "policyType"}
	}
	return nil
}

func policyReviewAuditRecords(records []Record) []Record {
	filtered := make([]Record, 0, len(records))
	for _, record := range records {
		if record.Values["provider"] == "policy-review" || strings.HasPrefix(record.Values["action"], "policy-review.") {
			filtered = append(filtered, cloneRecord(record))
		}
	}
	return filtered
}

func recordIndexByID(records []Record, id string) int {
	for index, record := range records {
		if record.ID == id {
			return index
		}
	}
	return -1
}

func recordIndexByCode(records []Record, code string) int {
	for index, record := range records {
		if record.Code == code {
			return index
		}
	}
	return -1
}
