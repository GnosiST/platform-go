package sensitivemigration

import (
	"context"
	"errors"
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
	RunID    string
	PlanHash string
	Plan     Plan
}

type RunState struct {
	RunID            string
	PlanHash         string
	Status           string
	ExpectedRevision uint64
	TargetCount      int
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
}

type BatchCommit struct {
	Revision      uint64
	Rows          int
	LastRecordID  string
	EventSequence uint64
}

type Options struct {
	Mode      Mode
	BatchSize int
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
