package datalifecycle

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/capability"
)

type Mode string

const (
	ModeImpact          Mode = "impact"
	ModeDryRun          Mode = "dry-run"
	ModeApply           Mode = "apply"
	StatusDisabled           = "disabled"
	StatusCompleted          = "completed"
	StatusFailed             = "failed"
	DefaultDatasourceID      = "default"
	DefaultBatchSize         = 100
	MaximumBatchSize         = 1000
	MaximumRetries           = 5
)

var (
	ErrDisabled                     = errors.New("data lifecycle runner disabled")
	ErrInvalidOptions               = errors.New("data lifecycle invalid options")
	ErrInvalidPolicy                = errors.New("data lifecycle invalid policy")
	ErrPersistentRepositoryRequired = errors.New("data lifecycle persistent repository required")
	ErrLeaseHeld                    = errors.New("data lifecycle lease held")
	ErrLeaseLost                    = errors.New("data lifecycle lease lost")
	ErrRepositoryFailed             = errors.New("data lifecycle repository failed")
	ErrPlanningFailed               = errors.New("data lifecycle planning failed")
	ErrApplyFailed                  = errors.New("data lifecycle apply failed")
	ErrPolicyDrift                  = errors.New("data lifecycle policy drift")
	ErrPromotionRequired            = errors.New("data lifecycle promotion required")
)

type PolicySnapshot struct {
	Version   uint32
	Resources []ResourcePolicy
}

type ResourcePolicy struct {
	Resource      string
	Mode          string
	PolicyVersion uint32
	RetentionDays int
	AutoPurge     bool
}

type canonicalPolicy struct {
	Version   uint32                    `json:"version"`
	Resources []canonicalResourcePolicy `json:"resources"`
}

type canonicalResourcePolicy struct {
	Resource      string `json:"resource"`
	Mode          string `json:"mode"`
	PolicyVersion uint32 `json:"policyVersion"`
	RetentionDays int    `json:"retentionDays"`
	AutoPurge     bool   `json:"autoPurge"`
}

func PolicyFingerprint(policy PolicySnapshot) (string, error) {
	if policy.Version == 0 || len(policy.Resources) == 0 {
		return "", ErrInvalidPolicy
	}
	resources := make([]canonicalResourcePolicy, 0, len(policy.Resources))
	seen := make(map[string]struct{}, len(policy.Resources))
	for _, resource := range policy.Resources {
		name := strings.TrimSpace(resource.Resource)
		mode := strings.TrimSpace(resource.Mode)
		if name == "" || name != resource.Resource || mode == "" || mode != resource.Mode || !capability.IsAdminDeletionMode(mode) ||
			resource.PolicyVersion == 0 {
			return "", ErrInvalidPolicy
		}
		if _, ok := capability.AdminRetentionDuration(resource.RetentionDays); !ok {
			return "", ErrInvalidPolicy
		}
		if resource.AutoPurge && (resource.RetentionDays == 0 || !capability.SupportsAdminAutoPurge(name, mode)) {
			return "", ErrInvalidPolicy
		}
		if _, exists := seen[name]; exists {
			return "", ErrInvalidPolicy
		}
		seen[name] = struct{}{}
		resources = append(resources, canonicalResourcePolicy{
			Resource: name, Mode: mode, PolicyVersion: resource.PolicyVersion,
			RetentionDays: resource.RetentionDays, AutoPurge: resource.AutoPurge,
		})
	}
	slices.SortFunc(resources, func(left, right canonicalResourcePolicy) int {
		return strings.Compare(left.Resource, right.Resource)
	})
	payload, err := json.Marshal(canonicalPolicy{Version: policy.Version, Resources: resources})
	if err != nil {
		return "", ErrInvalidPolicy
	}
	digest := sha256.Sum256(append([]byte("platform-go:data-lifecycle:policy:v1\x00"), payload...))
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

type Cursor struct {
	DatasourceID string `json:"datasourceId"`
	Resource     string `json:"resource,omitempty"`
	EligibleAt   string `json:"eligibleAt,omitempty"`
	RecordID     string `json:"recordId,omitempty"`
}

type Counts struct {
	Eligible int `json:"eligible"`
	Planned  int `json:"planned"`
	Applied  int `json:"applied"`
	Batches  int `json:"batches"`
	Retries  int `json:"retries"`
}

func (c Counts) plus(other Counts) Counts {
	return Counts{
		Eligible: c.Eligible + other.Eligible,
		Planned:  c.Planned + other.Planned,
		Applied:  c.Applied + other.Applied,
		Batches:  c.Batches + other.Batches,
		Retries:  c.Retries + other.Retries,
	}
}

type PromotionApproval struct {
	ImpactReportHash    string
	ActorID             string
	Reason              string
	ApprovalRef         string
	CurrentFingerprint  string
	PromotedFingerprint string
	DryRunID            string
}

type Options struct {
	Enabled           bool
	Mode              Mode
	RunID             string
	OwnerID           string
	DatasourceID      string
	BatchSize         int
	MaxRetries        int
	LeaseTTL          time.Duration
	HeartbeatInterval time.Duration
	OperationTimeout  time.Duration
	Policy            PolicySnapshot
	PolicyFingerprint string
	Promotion         PromotionApproval
}

type ReportFailure struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

type Report struct {
	DatasourceID      string          `json:"datasourceId"`
	RunID             string          `json:"runId,omitempty"`
	Mode              Mode            `json:"mode,omitempty"`
	Status            string          `json:"status"`
	Counts            Counts          `json:"counts"`
	Cursor            Cursor          `json:"cursor"`
	PolicyFingerprint string          `json:"policyFingerprint,omitempty"`
	EvidenceHash      string          `json:"evidenceHash,omitempty"`
	ImpactReportHash  string          `json:"impactReportHash,omitempty"`
	Failures          []ReportFailure `json:"failures,omitempty"`
}

type Candidate struct {
	Resource   string
	EligibleAt string
	RecordID   string
}

type PlanRequest struct {
	DatasourceID      string
	Mode              Mode
	RunID             string
	Policy            PolicySnapshot
	PolicyFingerprint string
	Cursor            Cursor
	Limit             int
}

type Batch struct {
	DatasourceID string
	Candidates   []Candidate
	NextCursor   Cursor
	Done         bool
}

type ApplyRequest struct {
	DatasourceID       string
	RunID              string
	BatchID            string
	Policy             PolicySnapshot
	PolicyFingerprint  string
	Lease              Lease
	Batch              Batch
	PreviousCheckpoint Checkpoint
	Checkpoint         Checkpoint
}

type ApplyResult struct {
	Applied    int
	Checkpoint Checkpoint
}

type CheckpointKey struct {
	DatasourceID      string
	RunID             string
	Mode              Mode
	PolicyFingerprint string
}

type Checkpoint struct {
	Key           CheckpointKey
	DatasourceID  string
	Cursor        Cursor
	Counts        Counts
	EvidenceHash  string
	IntegrityHash string
	LastBatchID   string
	Revision      uint64
	Complete      bool
	UpdatedAt     time.Time
}

type LeaseRequest struct {
	DatasourceID      string
	Key               string
	OwnerID           string
	PolicyFingerprint string
	Now               time.Time
	TTL               time.Duration
}

type Lease struct {
	DatasourceID      string
	Key               string
	OwnerID           string
	Token             string
	PolicyFingerprint string
	AcquiredAt        time.Time
	HeartbeatAt       time.Time
	ExpiresAt         time.Time
}

func (l Lease) Active(now time.Time) bool {
	return l.DatasourceID != "" && l.Key != "" && l.OwnerID != "" && l.Token != "" &&
		!l.ExpiresAt.IsZero() && now.Before(l.ExpiresAt)
}

type ImpactReport struct {
	DatasourceID      string
	RunID             string
	PolicyFingerprint string
	Counts            Counts
	Cursor            Cursor
	EvidenceHash      string
	ReportHash        string
	GeneratedAt       time.Time
}

type Promotion struct {
	DatasourceID        string
	CurrentFingerprint  string
	PromotedFingerprint string
	ImpactReportHash    string
	ActorID             string
	Reason              string
	ApprovalRef         string
	PromotedAt          time.Time
}

type PromotionRequest struct {
	Enabled             bool
	DatasourceID        string
	CurrentPolicy       PolicySnapshot
	ProposedPolicy      PolicySnapshot
	ImpactReportHash    string
	ActorID             string
	Reason              string
	ApprovalRef         string
	PromotedFingerprint string
}

type Repository interface {
	Persistent() bool
	AcquireLease(context.Context, LeaseRequest) (Lease, error)
	HeartbeatLease(context.Context, Lease, time.Time, time.Duration) (Lease, error)
	ReleaseLease(context.Context, Lease) error
	LoadCheckpoint(context.Context, CheckpointKey) (Checkpoint, bool, error)
	SaveCheckpoint(context.Context, Lease, Checkpoint) error
	SaveImpactReportAndCheckpoint(context.Context, Lease, ImpactReport, Checkpoint) error
	LoadImpactReport(context.Context, string, string) (ImpactReport, bool, error)
	SavePromotion(context.Context, Promotion) error
	LoadPromotion(context.Context, string, string) (Promotion, bool, error)
}

type BatchPlanner interface {
	Plan(context.Context, PlanRequest) (Batch, error)
}

type BatchApplier interface {
	ApplyAndCheckpoint(context.Context, ApplyRequest) (ApplyResult, error)
}

type Clock interface {
	Now() time.Time
}

func checkpointKey(options Options) CheckpointKey {
	datasourceID := options.DatasourceID
	if datasourceID == "" {
		datasourceID = DefaultDatasourceID
	}
	return CheckpointKey{
		DatasourceID: datasourceID, RunID: options.RunID, Mode: options.Mode,
		PolicyFingerprint: options.PolicyFingerprint,
	}
}

func canonicalDigest(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, character := range value[len("sha256:"):] {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}

func stableDigest(domain string, values ...string) string {
	digest := sha256.New()
	for _, value := range append([]string{"platform-go:data-lifecycle:v1", domain}, values...) {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		_, _ = digest.Write(size[:])
		_, _ = digest.Write([]byte(value))
	}
	return "sha256:" + hex.EncodeToString(digest.Sum(nil))
}

func sealCheckpoint(checkpoint Checkpoint) Checkpoint {
	checkpoint.IntegrityHash = checkpointDigest(checkpoint)
	return checkpoint
}

func checkpointDigest(checkpoint Checkpoint) string {
	return stableDigest(
		"checkpoint",
		checkpoint.Key.DatasourceID,
		checkpoint.Key.RunID,
		string(checkpoint.Key.Mode),
		checkpoint.Key.PolicyFingerprint,
		checkpoint.DatasourceID,
		checkpoint.Cursor.DatasourceID,
		checkpoint.Cursor.Resource,
		checkpoint.Cursor.EligibleAt,
		checkpoint.Cursor.RecordID,
		strconv.Itoa(checkpoint.Counts.Eligible),
		strconv.Itoa(checkpoint.Counts.Planned),
		strconv.Itoa(checkpoint.Counts.Applied),
		strconv.Itoa(checkpoint.Counts.Batches),
		strconv.Itoa(checkpoint.Counts.Retries),
		checkpoint.EvidenceHash,
		checkpoint.LastBatchID,
		strconv.FormatUint(checkpoint.Revision, 10),
		strconv.FormatBool(checkpoint.Complete),
	)
}
