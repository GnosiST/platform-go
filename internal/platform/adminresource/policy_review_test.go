package adminresource

import (
	"testing"

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

	result, err := store.ApprovePolicyReview(review.ID, "admin")
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
	audit := recordByCode(audits, "policy-review:PR-1001:approved")
	if audit == nil {
		t.Fatalf("audit logs = %+v, want policy review approval audit", audits)
	}
	if audit.Values["action"] != "policy-review.approve" || audit.Values["resource"] != "roles" || audit.Values["targetCode"] != "operator" {
		t.Fatalf("audit values = %+v, want role policy approval audit", audit.Values)
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

	requested, err := store.RequestPolicyReview(review.ID, "ops")
	if err != nil {
		t.Fatalf("RequestPolicyReview() error = %v", err)
	}
	if requested.Review.Values["reviewStatus"] != "pending" || requested.Review.Values["requestedBy"] != "ops" || requested.Review.Values["submittedAt"] == "" {
		t.Fatalf("requested review values = %+v, want pending request metadata", requested.Review.Values)
	}
	if requested.Audit.Values["action"] != "policy-review.request" || requested.Audit.Values["targetCode"] != "PR-1002" {
		t.Fatalf("request audit values = %+v, want policy-review.request audit", requested.Audit.Values)
	}

	rejected, err := store.RejectPolicyReview(review.ID, "admin", "too broad for operator")
	if err != nil {
		t.Fatalf("RejectPolicyReview() error = %v", err)
	}
	if rejected.Review.Values["reviewStatus"] != "rejected" || rejected.Review.Values["reviewedBy"] != "admin" || rejected.Review.Values["rejectionReason"] != "too broad for operator" {
		t.Fatalf("rejected review values = %+v, want rejected metadata", rejected.Review.Values)
	}
	if rejected.Audit.Values["action"] != "policy-review.reject" || rejected.Audit.Values["targetCode"] != "PR-1002" {
		t.Fatalf("reject audit values = %+v, want policy-review.reject audit", rejected.Audit.Values)
	}

	exported, err := store.ExportPolicyReviews("auditor")
	if err != nil {
		t.Fatalf("ExportPolicyReviews() error = %v", err)
	}
	if exported.ExportedBy != "auditor" || exported.ExportedAt == "" || len(exported.Reviews) == 0 {
		t.Fatalf("export metadata = %+v, want exported reviews", exported)
	}
	if !hasRecordCode(exported.Reviews, "PR-1002") {
		t.Fatalf("exported reviews = %+v, want PR-1002", exported.Reviews)
	}
	if !hasRecordCode(exported.Audits, "policy-review:PR-1002:requested") || !hasRecordCode(exported.Audits, "policy-review:PR-1002:rejected") {
		t.Fatalf("exported audits = %+v, want request and reject audits", exported.Audits)
	}
	if !hasRecordCode(exported.Audits, "policy-review:export") {
		t.Fatalf("exported audits = %+v, want export audit", exported.Audits)
	}
}
