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
	ExportedBy string   `json:"exportedBy"`
	ExportedAt string   `json:"exportedAt"`
	Reviews    []Record `json:"reviews"`
	Audits     []Record `json:"audits"`
}

func (s *Store) RequestPolicyReview(reviewID string, requesterCode string) (PolicyReviewActionResult, error) {
	return s.transitionPolicyReview(reviewID, policyReviewTransition{
		actorCode:   requesterCode,
		fromStatus:  "draft",
		toStatus:    "pending",
		auditSuffix: "requested",
		auditAction: "policy-review.request",
		auditName:   "Policy review requested",
		apply: func(review *Record, actorCode string, now string) {
			review.Values["requestedBy"] = strings.TrimSpace(actorCode)
			review.Values["submittedAt"] = now
		},
	})
}

func (s *Store) ApprovePolicyReview(reviewID string, reviewerCode string) (PolicyReviewResult, error) {
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

	s.nextID++
	audit := Record{
		ID:          fmt.Sprintf("audit-logs-%d", s.nextID),
		Code:        fmt.Sprintf("policy-review:%s:approved", review.Code),
		Name:        "Policy review approved",
		Status:      "recorded",
		Description: fmt.Sprintf("Policy review %s approved and applied to role %s.", review.Code, role.Code),
		UpdatedAt:   now,
		Values: map[string]string{
			"actor":      strings.TrimSpace(reviewerCode),
			"action":     "policy-review.approve",
			"resource":   "roles",
			"targetId":   role.ID,
			"targetCode": role.Code,
			"targetName": role.Name,
			"provider":   "policy-review",
			"traceId":    review.Code,
		},
	}
	audits = append(audits, audit)
	s.resources["audit-logs"] = audits

	if err := s.persistLocked(); err != nil {
		s.restoreSnapshotLocked(previous)
		return PolicyReviewResult{}, err
	}

	return PolicyReviewResult{Review: cloneRecord(updatedReview), Role: cloneRecord(role), Audit: cloneRecord(audit)}, nil
}

func (s *Store) RejectPolicyReview(reviewID string, reviewerCode string, reason string) (PolicyReviewActionResult, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return PolicyReviewActionResult{}, ValidationError{Field: "reason"}
	}
	return s.transitionPolicyReview(reviewID, policyReviewTransition{
		actorCode:   reviewerCode,
		fromStatus:  "pending",
		toStatus:    "rejected",
		auditSuffix: "rejected",
		auditAction: "policy-review.reject",
		auditName:   "Policy review rejected",
		apply: func(review *Record, actorCode string, now string) {
			review.Values["reviewedBy"] = strings.TrimSpace(actorCode)
			review.Values["reviewedAt"] = now
			review.Values["rejectionReason"] = reason
		},
	})
}

func (s *Store) ExportPolicyReviews(actorCode string) (PolicyReviewExport, error) {
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

	now := s.now().UTC().Format("2006-01-02T15:04:05Z07:00")
	audit := policyReviewAuditRecord(s.nextID+1, "export", "Policy review export", "policy-review.export", "policy-review:export", "Policy review ledger exported.", strings.TrimSpace(actorCode), Record{}, now)
	audit.Values["targetCode"] = "policy-reviews"
	audit.Values["targetName"] = "Policy Reviews"
	s.nextID++
	audits = append(audits, audit)
	s.resources["audit-logs"] = audits
	if err := s.persistLocked(); err != nil {
		s.restoreSnapshotLocked(previous)
		return PolicyReviewExport{}, err
	}

	return PolicyReviewExport{
		ExportedBy: strings.TrimSpace(actorCode),
		ExportedAt: now,
		Reviews:    cloneRecords(reviews),
		Audits:     policyReviewAuditRecords(audits),
	}, nil
}

type policyReviewTransition struct {
	actorCode   string
	fromStatus  string
	toStatus    string
	auditSuffix string
	auditAction string
	auditName   string
	apply       func(review *Record, actorCode string, now string)
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

	s.nextID++
	audit := policyReviewAuditRecord(s.nextID, transition.auditSuffix, transition.auditName, transition.auditAction, fmt.Sprintf("policy-review:%s:%s", review.Code, transition.auditSuffix), fmt.Sprintf("Policy review %s moved to %s.", review.Code, transition.toStatus), strings.TrimSpace(transition.actorCode), review, now)
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

func policyReviewAuditRecord(nextID int, suffix string, name string, action string, code string, description string, actorCode string, review Record, now string) Record {
	targetCode := review.Code
	targetName := review.Name
	targetID := review.ID
	if suffix == "" {
		suffix = "recorded"
	}
	return Record{
		ID:          fmt.Sprintf("audit-logs-%d", nextID),
		Code:        code,
		Name:        name,
		Status:      "recorded",
		Description: description,
		UpdatedAt:   now,
		Values: map[string]string{
			"actor":      actorCode,
			"action":     action,
			"resource":   "policy-reviews",
			"targetId":   targetID,
			"targetCode": targetCode,
			"targetName": targetName,
			"provider":   "policy-review",
			"traceId":    targetCode,
		},
	}
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
