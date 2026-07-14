package adminresource

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"platform-go/internal/platform/capability"
)

const (
	fileDeletionStateField       = "deletionState"
	fileDeletionRequestedAtField = "deletionRequestedAt"
	fileDeletionPending          = "pending"
	fileDeletionCleanupStarted   = "cleanup-started"
	fileDeletionObjectDeleted    = "object-deleted"
	FileDeletionPending          = fileDeletionPending
	FileDeletionCleanupStarted   = fileDeletionCleanupStarted
	FileDeletionObjectDeleted    = fileDeletionObjectDeleted
)

func FileDeletionState(record Record) string {
	return fileDeletionState(record)
}

func FileObjectKey(record Record) string {
	if key := strings.TrimSpace(record.Values["storageKey"]); key != "" {
		return key
	}
	return strings.TrimSpace(record.Code)
}

func FileStorageDriver(record Record) string {
	return strings.TrimSpace(record.Values["storageDriver"])
}

type AuditEvent struct {
	Actor      string
	Action     string
	Resource   string
	TargetID   string
	Result     string
	EventID    string
	ReasonCode string
}

type MutationResult struct {
	Record Record
	Audit  Record
}

func (s *Store) RecordAudit(event AuditEvent) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.prepareMutationLocked()
	if err != nil {
		return Record{}, err
	}
	audit, err := s.auditRecordLocked(event, s.nextID+1)
	if err != nil {
		return Record{}, err
	}
	s.nextID++
	s.resources["audit-logs"] = append(s.resources["audit-logs"], audit)
	if err := s.persistLocked(); err != nil {
		s.restoreSnapshotLocked(previous)
		return Record{}, err
	}
	return cloneRecord(audit), nil
}

func (s *Store) CreateWithAudit(resource string, input WriteInput, event AuditEvent) (MutationResult, error) {
	return s.createWithAudit(resource, input, WriteOriginExternal, event)
}

func (s *Store) CreateInternalWithAudit(resource string, input WriteInput, event AuditEvent) (MutationResult, error) {
	return s.createWithAudit(resource, input, WriteOriginInternal, event)
}

func (s *Store) createWithAudit(resource string, input WriteInput, origin WriteOrigin, event AuditEvent) (MutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.prepareMutationLocked()
	if err != nil {
		return MutationResult{}, err
	}
	items, ok := s.resources[resource]
	if !ok {
		return MutationResult{}, ErrUnknownResource
	}
	nextRecordID := s.nextID + 1
	record, err := s.recordFromInputWithOrigin(resource, fmt.Sprintf("%s-%d", resource, nextRecordID), input, origin)
	if err != nil {
		return MutationResult{}, err
	}
	if err := s.protectRecordForStorage(context.Background(), resource, &record, nil); err != nil {
		return MutationResult{}, err
	}
	event.TargetID = record.ID
	event.Resource = resource
	audit, err := s.auditRecordLocked(event, s.nextID+2)
	if err != nil {
		return MutationResult{}, err
	}
	resultRecord, err := s.mutationRecordResultLocked(resource, record, origin)
	if err != nil {
		return MutationResult{}, err
	}
	s.nextID += 2
	s.resources[resource] = append(items, record)
	s.resources["audit-logs"] = append(s.resources["audit-logs"], audit)
	if err := s.persistLocked(); err != nil {
		s.restoreSnapshotLocked(previous)
		return MutationResult{}, err
	}
	return MutationResult{Record: resultRecord, Audit: cloneRecord(audit)}, nil
}

func (s *Store) UpdateWithAudit(resource string, id string, input WriteInput, event AuditEvent) (MutationResult, error) {
	return s.updateWithAudit(resource, id, input, WriteOriginExternal, event)
}

func (s *Store) UpdateInternalWithAudit(resource string, id string, input WriteInput, event AuditEvent) (MutationResult, error) {
	return s.updateWithAudit(resource, id, input, WriteOriginInternal, event)
}

func (s *Store) updateWithAudit(resource string, id string, input WriteInput, origin WriteOrigin, event AuditEvent) (MutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.prepareMutationLocked()
	if err != nil {
		return MutationResult{}, err
	}
	items, ok := s.resources[resource]
	if !ok {
		return MutationResult{}, ErrUnknownResource
	}
	index := recordIndexByID(items, id)
	if index < 0 {
		return MutationResult{}, ErrRecordNotFound
	}
	if strings.TrimSpace(input.Code) == "" {
		input.Code = items[index].Code
	}
	record, err := s.recordFromInputWithOriginExisting(resource, id, input, origin, &items[index])
	if err != nil {
		return MutationResult{}, err
	}
	if err := s.protectRecordForStorage(context.Background(), resource, &record, &items[index]); err != nil {
		return MutationResult{}, err
	}
	event.TargetID = record.ID
	event.Resource = resource
	audit, err := s.auditRecordLocked(event, s.nextID+1)
	if err != nil {
		return MutationResult{}, err
	}
	resultRecord, err := s.mutationRecordResultLocked(resource, record, origin)
	if err != nil {
		return MutationResult{}, err
	}
	items[index] = record
	s.nextID++
	s.resources[resource] = items
	s.resources["audit-logs"] = append(s.resources["audit-logs"], audit)
	if err := s.persistLocked(); err != nil {
		s.restoreSnapshotLocked(previous)
		return MutationResult{}, err
	}
	return MutationResult{Record: resultRecord, Audit: cloneRecord(audit)}, nil
}

func (s *Store) DeleteWithAudit(resource string, id string, event AuditEvent) (MutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.prepareMutationLocked()
	if err != nil {
		return MutationResult{}, err
	}
	record, nextItems, err := s.deletionMutationLocked(resource, id, event.Actor, event.ReasonCode)
	if err != nil {
		return MutationResult{}, err
	}
	event.TargetID = record.ID
	event.Resource = resource
	audit, err := s.auditRecordLocked(event, s.nextID+1)
	if err != nil {
		return MutationResult{}, err
	}
	s.nextID++
	s.resources[resource] = nextItems
	s.resources["audit-logs"] = append(s.resources["audit-logs"], audit)
	if err := s.persistLocked(); err != nil {
		s.restoreSnapshotLocked(previous)
		return MutationResult{}, err
	}
	return MutationResult{Record: record, Audit: cloneRecord(audit)}, nil
}

func (s *Store) RestoreWithAudit(resource string, id string, event AuditEvent) (MutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.prepareMutationLocked()
	if err != nil {
		return MutationResult{}, err
	}
	record, nextItems, err := s.restoreMutationLocked(resource, id)
	if err != nil {
		return MutationResult{}, err
	}
	event.TargetID = record.ID
	event.Resource = resource
	audit, err := s.auditRecordLocked(event, s.nextID+1)
	if err != nil {
		return MutationResult{}, err
	}
	resultRecord, err := s.mutationRecordResultLocked(resource, record, WriteOriginExternal)
	if err != nil {
		return MutationResult{}, err
	}
	s.nextID++
	s.resources[resource] = nextItems
	s.resources["audit-logs"] = append(s.resources["audit-logs"], audit)
	if err := s.persistLocked(); err != nil {
		s.restoreSnapshotLocked(previous)
		return MutationResult{}, err
	}
	return MutationResult{Record: resultRecord, Audit: cloneRecord(audit)}, nil
}

func (s *Store) PurgeWithAudit(resource string, id string, event AuditEvent) (MutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.prepareMutationLocked()
	if err != nil {
		return MutationResult{}, err
	}
	record, nextItems, err := s.purgeMutationLocked(resource, id)
	if err != nil {
		return MutationResult{}, err
	}
	event.TargetID = record.ID
	event.Resource = resource
	audit, err := s.auditRecordLocked(event, s.nextID+1)
	if err != nil {
		return MutationResult{}, err
	}
	s.nextID++
	s.resources[resource] = nextItems
	s.resources["audit-logs"] = append(s.resources["audit-logs"], audit)
	if err := s.persistLocked(); err != nil {
		s.restoreSnapshotLocked(previous)
		return MutationResult{}, err
	}
	return MutationResult{Record: record, Audit: cloneRecord(audit)}, nil
}

func (s *Store) PurgeWithPolicyAndAudit(resource string, id string, policy MaintenanceRetentionPolicy, event AuditEvent) (MutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.prepareMutationLocked()
	if err != nil {
		return MutationResult{}, err
	}
	record, nextItems, err := s.purgeMutationWithPolicyLocked(resource, id, policy)
	if err != nil {
		return MutationResult{}, err
	}
	event.TargetID = record.ID
	event.Resource = resource
	audit, err := s.auditRecordLocked(event, s.nextID+1)
	if err != nil {
		return MutationResult{}, err
	}
	s.nextID++
	s.resources[resource] = nextItems
	s.resources["audit-logs"] = append(s.resources["audit-logs"], audit)
	if err := s.persistLocked(); err != nil {
		s.restoreSnapshotLocked(previous)
		return MutationResult{}, err
	}
	return MutationResult{Record: record, Audit: cloneRecord(audit)}, nil
}

func (s *Store) PurgeRevokedWithPolicyAndAudit(resource string, id string, terminalField string, policy MaintenanceRetentionPolicy, event AuditEvent) (MutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.prepareMutationLocked()
	if err != nil {
		return MutationResult{}, err
	}
	items, ok := s.resources[resource]
	if !ok {
		return MutationResult{}, ErrUnknownResource
	}
	index := recordIndexByID(items, id)
	if index < 0 {
		return MutationResult{}, ErrRecordNotFound
	}
	configured, err := s.deletionPolicyLocked(resource)
	if err != nil {
		return MutationResult{}, err
	}
	retention, ok := capability.AdminRetentionDuration(policy.RetentionDays)
	if configured.Mode != capability.AdminDeletionRevoke || !configured.AutoPurge || !policy.AutoPurge ||
		policy.Mode != capability.AdminDeletionRevoke || !capability.SupportsAdminAutoPurge(resource, policy.Mode) || !ok || retention <= 0 ||
		configured.PolicyVersion != policy.PolicyVersion || configured.RetentionDays != policy.RetentionDays || policy.RetentionDays <= 0 {
		return MutationResult{}, ErrRetentionPolicyMismatch
	}
	record := cloneRecord(items[index])
	if record.Status != "revoked" {
		return MutationResult{}, ErrRecordNotDeleted
	}
	terminalAt, err := time.Parse(time.RFC3339, strings.TrimSpace(record.Values[terminalField]))
	if err != nil {
		return MutationResult{}, fmt.Errorf("%w: invalid terminal timestamp", ErrInvalidRecord)
	}
	if s.now().UTC().Before(terminalAt.UTC().Add(retention)) {
		return MutationResult{}, ErrRetentionNotElapsed
	}
	event.TargetID = record.ID
	event.Resource = resource
	audit, err := s.auditRecordLocked(event, s.nextID+1)
	if err != nil {
		return MutationResult{}, err
	}
	s.nextID++
	s.resources[resource] = slices.Delete(append([]Record(nil), items...), index, index+1)
	s.resources["audit-logs"] = append(s.resources["audit-logs"], audit)
	if err := s.persistLocked(); err != nil {
		s.restoreSnapshotLocked(previous)
		return MutationResult{}, err
	}
	return MutationResult{Record: record, Audit: cloneRecord(audit)}, nil
}

func (s *Store) TombstoneFileWithAudit(id string, event AuditEvent) (MutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.prepareMutationLocked()
	if err != nil {
		return MutationResult{}, err
	}
	items, ok := s.resources["files"]
	if !ok {
		return MutationResult{}, ErrUnknownResource
	}
	index := recordIndexByID(items, id)
	if index < 0 {
		return MutationResult{}, ErrRecordNotFound
	}
	record := cloneRecord(items[index])
	if isTombstonedFile(record) {
		return MutationResult{Record: record}, nil
	}
	policy, err := s.deletionPolicyLocked("files")
	if err != nil {
		return MutationResult{}, err
	}
	if policy.Mode != capability.AdminDeletionTombstone {
		return MutationResult{}, ErrDeletionRequiresAdapter
	}
	retention, ok := capability.AdminRetentionDuration(policy.RetentionDays)
	if !ok {
		return MutationResult{}, ErrDeletionPolicyMissing
	}
	if record.Values == nil {
		record.Values = map[string]string{}
	}
	now := s.now().UTC()
	record.Values[fileDeletionStateField] = fileDeletionPending
	record.Values[fileDeletionRequestedAtField] = now.Format(time.RFC3339)
	record.DeletedAt = now.Format(time.RFC3339)
	record.DeletedBy = strings.TrimSpace(event.Actor)
	if record.DeletedBy == "" {
		record.DeletedBy = "system"
	}
	record.DeleteReason = strings.TrimSpace(event.ReasonCode)
	if record.DeleteReason == "" {
		record.DeleteReason = "cleanup-pending"
	}
	record.DeletionPolicyVersion = policy.PolicyVersion
	if policy.RetentionDays > 0 {
		record.PurgeAfter = now.Add(retention).Format(time.RFC3339)
	}
	record.UpdatedAt = now.Format(time.RFC3339)
	event.TargetID = record.ID
	event.Resource = "files"
	audit, err := s.auditRecordLocked(event, s.nextID+1)
	if err != nil {
		return MutationResult{}, err
	}
	items[index] = record
	s.nextID++
	s.resources["files"] = items
	s.resources["audit-logs"] = append(s.resources["audit-logs"], audit)
	if err := s.persistLocked(); err != nil {
		s.restoreSnapshotLocked(previous)
		return MutationResult{}, err
	}
	return MutationResult{Record: cloneRecord(record), Audit: cloneRecord(audit)}, nil
}

func (s *Store) PurgeTombstonedFileWithAudit(id string, event AuditEvent) (MutationResult, error) {
	return s.purgeTombstonedFileWithAudit(id, nil, event)
}

func (s *Store) PurgeTombstonedFileWithPolicyAndAudit(id string, policy MaintenanceRetentionPolicy, event AuditEvent) (MutationResult, error) {
	return s.purgeTombstonedFileWithAudit(id, &policy, event)
}

func (s *Store) purgeTombstonedFileWithAudit(id string, policy *MaintenanceRetentionPolicy, event AuditEvent) (MutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.prepareMutationLocked()
	if err != nil {
		return MutationResult{}, err
	}
	items, ok := s.resources["files"]
	if !ok {
		return MutationResult{}, ErrUnknownResource
	}
	index := recordIndexByID(items, id)
	if index < 0 {
		return MutationResult{}, ErrRecordNotFound
	}
	if fileDeletionState(items[index]) != fileDeletionObjectDeleted {
		return MutationResult{}, ErrDeletionCleanupStarted
	}
	var record Record
	var nextItems []Record
	if policy == nil {
		record, nextItems, err = s.purgeMutationLocked("files", id)
	} else {
		record, nextItems, err = s.purgeMutationWithPolicyLocked("files", id, *policy)
	}
	if err != nil {
		return MutationResult{}, err
	}
	event.TargetID = record.ID
	event.Resource = "files"
	audit, err := s.auditRecordLocked(event, s.nextID+1)
	if err != nil {
		return MutationResult{}, err
	}
	s.nextID++
	s.resources["files"] = nextItems
	s.resources["audit-logs"] = append(s.resources["audit-logs"], audit)
	if err := s.persistLocked(); err != nil {
		s.restoreSnapshotLocked(previous)
		return MutationResult{}, err
	}
	return MutationResult{Record: record, Audit: cloneRecord(audit)}, nil
}

func (s *Store) ClaimTombstonedFileCleanupWithAudit(id string, event AuditEvent) (MutationResult, error) {
	return s.claimTombstonedFileCleanupWithAudit(id, nil, event)
}

func (s *Store) ClaimTombstonedFileCleanupWithPolicyAndAudit(id string, policy MaintenanceRetentionPolicy, event AuditEvent) (MutationResult, error) {
	return s.claimTombstonedFileCleanupWithAudit(id, &policy, event)
}

func (s *Store) claimTombstonedFileCleanupWithAudit(id string, policy *MaintenanceRetentionPolicy, event AuditEvent) (MutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.prepareMutationLocked()
	if err != nil {
		return MutationResult{}, err
	}
	items, ok := s.resources["files"]
	if !ok {
		return MutationResult{}, ErrUnknownResource
	}
	index := recordIndexByID(items, id)
	if index < 0 {
		return MutationResult{}, ErrRecordNotFound
	}
	record := cloneRecord(items[index])
	state := fileDeletionState(record)
	if state == fileDeletionCleanupStarted || state == fileDeletionObjectDeleted {
		return MutationResult{Record: record}, nil
	}
	if state != fileDeletionPending {
		return MutationResult{}, ErrRecordNotDeleted
	}
	if policy == nil {
		if err := s.requireRetentionElapsedLocked(record); err != nil {
			return MutationResult{}, err
		}
	} else if err := s.requireMaintenanceRetentionElapsedLocked("files", record, *policy); err != nil {
		return MutationResult{}, err
	}
	record.Values[fileDeletionStateField] = fileDeletionCleanupStarted
	record.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	event.TargetID = record.ID
	event.Resource = "files"
	audit, err := s.auditRecordLocked(event, s.nextID+1)
	if err != nil {
		return MutationResult{}, err
	}
	items[index] = record
	s.nextID++
	s.resources["files"] = items
	s.resources["audit-logs"] = append(s.resources["audit-logs"], audit)
	if err := s.persistLocked(); err != nil {
		s.restoreSnapshotLocked(previous)
		return MutationResult{}, err
	}
	return MutationResult{Record: cloneRecord(record), Audit: cloneRecord(audit)}, nil
}

func (s *Store) CompleteTombstonedFileCleanupWithAudit(id string, event AuditEvent) (MutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.prepareMutationLocked()
	if err != nil {
		return MutationResult{}, err
	}
	items, ok := s.resources["files"]
	if !ok {
		return MutationResult{}, ErrUnknownResource
	}
	index := recordIndexByID(items, id)
	if index < 0 {
		return MutationResult{}, ErrRecordNotFound
	}
	record := cloneRecord(items[index])
	if fileDeletionState(record) == fileDeletionObjectDeleted {
		return MutationResult{Record: record}, nil
	}
	if fileDeletionState(record) != fileDeletionCleanupStarted {
		return MutationResult{}, ErrDeletionCleanupStarted
	}
	record.Values[fileDeletionStateField] = fileDeletionObjectDeleted
	record.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	event.TargetID = record.ID
	event.Resource = "files"
	audit, err := s.auditRecordLocked(event, s.nextID+1)
	if err != nil {
		return MutationResult{}, err
	}
	items[index] = record
	s.nextID++
	s.resources["files"] = items
	s.resources["audit-logs"] = append(s.resources["audit-logs"], audit)
	if err := s.persistLocked(); err != nil {
		s.restoreSnapshotLocked(previous)
		return MutationResult{}, err
	}
	return MutationResult{Record: cloneRecord(record), Audit: cloneRecord(audit)}, nil
}

func (s *Store) requireRetentionElapsedLocked(record Record) error {
	if strings.TrimSpace(record.PurgeAfter) == "" {
		return ErrRetentionNotConfigured
	}
	purgeAfter, err := time.Parse(time.RFC3339, record.PurgeAfter)
	if err != nil {
		return fmt.Errorf("%w: invalid purgeAfter", ErrInvalidRecord)
	}
	if s.now().UTC().Before(purgeAfter.UTC()) {
		return ErrRetentionNotElapsed
	}
	return nil
}

func (s *Store) InternalRecord(resource string, id string) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, ok := s.resources[resource]
	if !ok {
		return Record{}, ErrUnknownResource
	}
	index := recordIndexByID(items, id)
	if index < 0 {
		return Record{}, ErrRecordNotFound
	}
	return cloneRecord(items[index]), nil
}

func (s *Store) InternalRecordsContext(ctx context.Context, resource string) ([]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.reloadContextLocked(ctx); err != nil {
		return nil, err
	}
	items, ok := s.resources[resource]
	if !ok {
		return nil, ErrUnknownResource
	}
	return cloneRecords(items), nil
}

func (s *Store) auditRecordLocked(event AuditEvent, nextID int) (Record, error) {
	if _, ok := s.resources["audit-logs"]; !ok {
		return Record{}, ErrUnknownResource
	}
	event.Actor = strings.TrimSpace(event.Actor)
	if event.Actor == "" {
		event.Actor = "system"
	}
	event.Action = strings.TrimSpace(event.Action)
	event.Resource = strings.TrimSpace(event.Resource)
	event.TargetID = strings.TrimSpace(event.TargetID)
	if event.Action == "" || event.Resource == "" || event.TargetID == "" {
		return Record{}, ErrInvalidRecord
	}
	if strings.TrimSpace(event.Result) == "" {
		event.Result = "success"
	}
	if strings.TrimSpace(event.ReasonCode) == "" {
		event.ReasonCode = "completed"
	}
	if strings.TrimSpace(event.EventID) == "" {
		event.EventID = fmt.Sprintf("event-%d", nextID)
	}
	values := map[string]string{
		"actor": event.Actor, "action": event.Action, "resource": event.Resource,
		"targetId": event.TargetID, "outcome": strings.TrimSpace(event.Result),
		"eventId": strings.TrimSpace(event.EventID), "reasonCode": strings.TrimSpace(event.ReasonCode),
		"createdAt": s.now().UTC().Format(time.RFC3339),
	}
	record, err := s.recordFromInputWithOrigin("audit-logs", "", WriteInput{
		Code: fmt.Sprintf("audit-%d", nextID), Name: "Protected Operation", Status: "recorded",
		Description: "Protected operation audit record.", Values: values,
	}, WriteOriginInternal)
	if err != nil {
		return Record{}, err
	}
	record.ID = fmt.Sprintf("audit-logs-%d", nextID)
	if _, err := s.projectRecordLocked("audit-logs", record, ProjectionExport); err != nil {
		return Record{}, err
	}
	return record, nil
}

func isTombstonedFile(record Record) bool {
	switch fileDeletionState(record) {
	case fileDeletionPending, fileDeletionCleanupStarted, fileDeletionObjectDeleted:
		return true
	default:
		return false
	}
}

func fileDeletionState(record Record) string {
	return strings.TrimSpace(record.Values[fileDeletionStateField])
}

func visibleRecords(resource string, records []Record) []Record {
	visible := make([]Record, 0, len(records))
	for _, record := range records {
		if !isLifecycleDeleted(record) && (resource != "files" || !isTombstonedFile(record)) {
			visible = append(visible, record)
		}
	}
	return visible
}
