package adminresource

import (
	"testing"
	"time"

	"platform-go/internal/platform/core"
)

func TestApprovePolicyReviewAppliesRolePermissionChangeAndRecordsAudit(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())

	review, err := store.Create("policy-reviews", WriteInput{
		Code:        "PR-1001",
		Name:        "Grant user read to operator",
		Status:      "enabled",
		Description: "Request operator user-read access through policy review.",
		Values: map[string]string{
			"policyType":      "role_permission",
			"requestedAction": "update",
			"reviewStatus":    "pending",
			"roleCode":        "operator",
			"permissionCodes": "admin:user:read",
			"requestedBy":     "admin",
		},
	})
	if err != nil {
		t.Fatalf("Create(policy-reviews) error = %v", err)
	}

	result, err := store.ApprovePolicyReview(review.ID, "admin", "user-admin")
	if err != nil {
		t.Fatalf("ApprovePolicyReview() error = %v", err)
	}
	if result.Review.Values["reviewStatus"] != "approved" {
		t.Fatalf("reviewStatus = %q, want approved", result.Review.Values["reviewStatus"])
	}
	if result.Review.Values["reviewedBy"] != "admin" || result.Review.Values["reviewedAt"] == "" {
		t.Fatalf("reviewer fields = %+v, want reviewedBy and reviewedAt", result.Review.Values)
	}
	if result.Role.Code != "operator" || result.Role.Values["permissions"] != "admin:user:read" {
		t.Fatalf("role result = %+v, want operator permissions updated", result.Role)
	}

	roles, err := store.List("roles")
	if err != nil {
		t.Fatalf("List(roles) error = %v", err)
	}
	operator := recordByCode(roles, "operator")
	if operator == nil || operator.Values["permissions"] != "admin:user:read" {
		t.Fatalf("operator role = %+v, want approved permissions applied", operator)
	}

	audits, err := store.List("audit-logs")
	if err != nil {
		t.Fatalf("List(audit-logs) error = %v", err)
	}
	audit := recordByAction(audits, "policy-review.approve")
	if audit == nil {
		t.Fatalf("audit logs = %+v, want policy review approval audit", audits)
	}
	if audit.Values["action"] != "policy-review.approve" || audit.Values["resource"] != "roles" || audit.Values["targetId"] != result.Role.ID {
		t.Fatalf("audit values = %+v, want role policy approval audit", audit.Values)
	}
	if audit.Values["actor"] != "user-admin" {
		t.Fatalf("audit actor = %q, want stable user ID", audit.Values["actor"])
	}
}

func TestRequestRejectAndExportPolicyReviewWorkflow(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())

	review, err := store.Create("policy-reviews", WriteInput{
		Code:        "PR-1002",
		Name:        "Update operator deny permissions",
		Status:      "enabled",
		Description: "Draft policy review should move through request and rejection.",
		Values: map[string]string{
			"policyType":      "deny_permission",
			"requestedAction": "update",
			"reviewStatus":    "draft",
			"roleCode":        "operator",
			"permissionCodes": "admin:tenant:delete",
			"requestedBy":     "ops",
		},
	})
	if err != nil {
		t.Fatalf("Create(policy-reviews) error = %v", err)
	}

	requested, err := store.RequestPolicyReview(review.ID, "ops", "user-ops")
	if err != nil {
		t.Fatalf("RequestPolicyReview() error = %v", err)
	}
	if requested.Review.Values["reviewStatus"] != "pending" || requested.Review.Values["requestedBy"] != "ops" || requested.Review.Values["submittedAt"] == "" {
		t.Fatalf("requested review values = %+v, want pending request metadata", requested.Review.Values)
	}
	if requested.Audit.Values["action"] != "policy-review.request" || requested.Audit.Values["targetId"] != review.ID || requested.Audit.Values["actor"] != "user-ops" {
		t.Fatalf("request audit values = %+v, want policy-review.request audit", requested.Audit.Values)
	}

	rejected, err := store.RejectPolicyReview(review.ID, "admin", "user-admin", "too broad for operator")
	if err != nil {
		t.Fatalf("RejectPolicyReview() error = %v", err)
	}
	if rejected.Review.Values["reviewStatus"] != "rejected" || rejected.Review.Values["reviewedBy"] != "admin" || rejected.Review.Values["rejectionReason"] != "too broad for operator" {
		t.Fatalf("rejected review values = %+v, want rejected metadata", rejected.Review.Values)
	}
	if rejected.Audit.Values["action"] != "policy-review.reject" || rejected.Audit.Values["targetId"] != review.ID || rejected.Audit.Values["actor"] != "user-admin" {
		t.Fatalf("reject audit values = %+v, want policy-review.reject audit", rejected.Audit.Values)
	}

	exported, err := store.ExportPolicyReviews("auditor", "user-auditor", false)
	if err != nil {
		t.Fatalf("ExportPolicyReviews() error = %v", err)
	}
	if exported.ExportedBy != "auditor" || exported.ExportedAt == "" || len(exported.Reviews) == 0 {
		t.Fatalf("export metadata = %+v, want exported reviews", exported)
	}
	if !hasRecordCode(exported.Reviews, "PR-1002") {
		t.Fatalf("exported reviews = %+v, want PR-1002", exported.Reviews)
	}
	if recordByAction(exported.Audits, "policy-review.request") == nil || recordByAction(exported.Audits, "policy-review.reject") == nil {
		t.Fatalf("exported audits = %+v, want request and reject audits", exported.Audits)
	}
	if recordByAction(exported.Audits, "policy-review.export") == nil {
		t.Fatalf("exported audits = %+v, want export audit", exported.Audits)
	}
	if recordByAction(exported.Audits, "policy-review.export").Values["actor"] != "user-auditor" {
		t.Fatalf("export audit actor = %+v, want stable user ID", recordByAction(exported.Audits, "policy-review.export"))
	}
	if exported.Watermark.Applied {
		t.Fatalf("export watermark = %+v, want disabled", exported.Watermark)
	}
}

func TestExportPolicyReviewsAddsWatermarkProvenanceAndBooleanAuditValue(t *testing.T) {
	fixedNow := time.Date(2026, time.July, 12, 10, 30, 0, 0, time.UTC)
	store := NewStoreFromCapabilities(core.DefaultManifests())
	store.now = func() time.Time { return fixedNow }

	exported, err := store.ExportPolicyReviews("auditor", "opaque-user-id", true)
	if err != nil {
		t.Fatalf("ExportPolicyReviews() error = %v", err)
	}
	if !exported.Watermark.Applied || exported.Watermark.Product != "Platform Go" {
		t.Fatalf("watermark = %+v, want applied Platform Go provenance", exported.Watermark)
	}
	if exported.Watermark.ExportedBy != "auditor" || exported.Watermark.ExportedAt != fixedNow.Format(time.RFC3339) {
		t.Fatalf("watermark = %+v, want stable actor and time", exported.Watermark)
	}
	if exported.Watermark.ExportedBy != exported.ExportedBy || exported.Watermark.ExportedAt != exported.ExportedAt {
		t.Fatalf("watermark = %+v export = %+v, want matching export metadata", exported.Watermark, exported)
	}

	audit := recordByAction(exported.Audits, "policy-review.export")
	if audit == nil {
		t.Fatalf("exported audits = %+v, want export audit", exported.Audits)
	}
	if audit.Values["actor"] != "opaque-user-id" || audit.Values["reasonCode"] != "watermarkApplied=true" {
		t.Fatalf("export audit = %+v, want opaque actor and boolean watermark audit value", audit)
	}
	for _, key := range []string{"watermark", "product", "exportedBy", "exportedAt"} {
		if value := audit.Values[key]; value != "" {
			t.Fatalf("export audit %s = %q, want no free-form watermark metadata", key, value)
		}
	}
}

func recordByAction(records []Record, action string) *Record {
	for index := range records {
		if records[index].Values["action"] == action {
			return &records[index]
		}
	}
	return nil
}
