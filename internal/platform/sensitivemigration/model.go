package sensitivemigration

import (
	"context"
	"errors"
	"strings"
)

type Mode string

const (
	ModeInventory       Mode = "inventory"
	ModeDryRun          Mode = "dry-run"
	ModePrepare         Mode = "prepare"
	ModeApply           Mode = "apply"
	ModeVerify          Mode = "verify"
	ModeRehearseRestore Mode = "rehearse-restore"
	ModeRollback        Mode = "rollback"
	StatusCompleted          = "completed"
	StatusFailed             = "failed"
	StatusPrepared           = "prepared"
	DefaultBatchSize         = 100
	MaximumBatchSize         = 1000
)

var (
	ErrInvalidOptions = errors.New("sensitive migration invalid options")
	ErrReadFailed     = errors.New("sensitive migration read failed")
	ErrMutationFailed = errors.New("sensitive migration mutation failed")
	ErrVerifyFailed   = errors.New("sensitive migration verification failed")
	ErrRunConflict    = errors.New("sensitive migration run conflict")
)

type Cursor struct {
	TenantID string
	RecordID string
}

type Row struct {
	Resource   string
	RecordID   string
	ValuesJSON string
}

type ReadStore interface {
	TenantScopes(context.Context, ResourcePlan) ([]string, error)
	Rows(context.Context, ResourcePlan, string, string, int) ([]Row, error)
}

type RunRequest struct {
	RunID                string
	PlanHash             string
	Plan                 Plan
	Mode                 Mode
	ActorID              string
	Reason               string
	ApprovalRef          string
	BackupURI            string
	BackupHash           string
	RestoreEvidence      string
	MaintenanceConfirmed bool
}

type RunState struct {
	RunID            string
	PlanHash         string
	Status           string
	ExpectedRevision uint64
	TargetCount      int
	Counts           Counts
	Checkpoints      []CheckpointState
	EventChainHead   string
}

type CheckpointState struct {
	Resource     string
	TenantID     string
	LastRecordID string
	Counts       Counts
	Batches      int
	EventHash    string
}

type RowMutation struct {
	RecordID           string
	OriginalValuesJSON string
	UpdatedValuesJSON  string
}

type BatchMutation struct {
	RunID            string
	Mode             Mode
	Resource         ResourcePlan
	TenantID         string
	ExpectedRevision uint64
	LastRecordID     string
	Rows             []RowMutation
	Counts           Counts
}

type BatchCommit struct {
	Revision      uint64
	Rows          int
	LastRecordID  string
	EventSequence uint64
	EventHash     string
}

type MutatingStore interface {
	ReadStore
	Prepare(context.Context, RunRequest) (RunState, error)
	StartOrResume(context.Context, RunRequest) (RunState, error)
	TargetScopes(context.Context, string, ResourcePlan) ([]string, error)
	TargetRows(context.Context, string, ResourcePlan, string, string, int) ([]Row, error)
	ApplyBatch(context.Context, BatchMutation) (BatchCommit, error)
	FinishRun(context.Context, string, string) error
}

type Options struct {
	Mode      Mode
	BatchSize int
	Request   RunRequest
}

type Counts struct {
	Missing           int `json:"missing"`
	Plaintext         int `json:"plaintext"`
	TargetEnvelope    int `json:"targetEnvelope"`
	ForeignEnvelope   int `json:"foreignEnvelope"`
	MalformedEnvelope int `json:"malformedEnvelope"`
}

type Report struct {
	RunID          string `json:"runId,omitempty"`
	Mode           Mode   `json:"mode"`
	Status         string `json:"status"`
	Counts         Counts `json:"counts"`
	Checkpoints    int    `json:"checkpoints"`
	EventChainHead string `json:"eventChainHead,omitempty"`
}

func (c Counts) total() int {
	return c.Missing + c.Plaintext + c.TargetEnvelope + c.ForeignEnvelope + c.MalformedEnvelope
}

func (c Counts) plus(other Counts) Counts {
	return Counts{
		Missing: c.Missing + other.Missing, Plaintext: c.Plaintext + other.Plaintext,
		TargetEnvelope:    c.TargetEnvelope + other.TargetEnvelope,
		ForeignEnvelope:   c.ForeignEnvelope + other.ForeignEnvelope,
		MalformedEnvelope: c.MalformedEnvelope + other.MalformedEnvelope,
	}
}

func validRunIdentity(request RunRequest) bool {
	if !canonicalIdentifier(request.RunID, 64) || strings.TrimSpace(request.PlanHash) == "" || request.PlanHash != strings.TrimSpace(request.PlanHash) {
		return false
	}
	return true
}

func validMutationRequest(request RunRequest) bool {
	if !validRunIdentity(request) || !request.MaintenanceConfirmed || !canonicalSHA256(request.BackupHash) {
		return false
	}
	for _, value := range []string{request.ActorID, request.Reason, request.ApprovalRef, request.BackupURI, request.RestoreEvidence} {
		if strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) {
			return false
		}
	}
	return true
}

func ValidMutationRequest(request RunRequest) bool {
	return validMutationRequest(request)
}

func ValidRunIdentity(request RunRequest) bool {
	return validRunIdentity(request)
}

func canonicalIdentifier(value string, maximum int) bool {
	if value == "" || len(value) > maximum || value != strings.TrimSpace(value) {
		return false
	}
	for index, character := range []byte(value) {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || index > 0 && (character == '-' || character == '_' || character == '.') {
			continue
		}
		return false
	}
	return true
}

func canonicalSHA256(value string) bool {
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
