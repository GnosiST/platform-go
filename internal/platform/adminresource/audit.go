package adminresource

import (
	"fmt"
	"strings"
	"time"
)

const (
	fileDeletionStateField       = "deletionState"
	fileDeletionRequestedAtField = "deletionRequestedAt"
	fileDeletionPending          = "pending"
)

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
	record, err := s.recordFromInputWithOrigin(resource, "", input, origin)
	if err != nil {
		return MutationResult{}, err
	}
	record.ID = fmt.Sprintf("%s-%d", resource, s.nextID+1)
	event.TargetID = record.ID
	event.Resource = resource
	audit, err := s.auditRecordLocked(event, s.nextID+2)
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
	return MutationResult{Record: cloneRecord(record), Audit: cloneRecord(audit)}, nil
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
	record, err := s.recordFromInputWithOrigin(resource, id, input, origin)
	if err != nil {
		return MutationResult{}, err
	}
	event.TargetID = record.ID
	event.Resource = resource
	audit, err := s.auditRecordLocked(event, s.nextID+1)
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
	return MutationResult{Record: cloneRecord(record), Audit: cloneRecord(audit)}, nil
}

func (s *Store) DeleteWithAudit(resource string, id string, event AuditEvent) (MutationResult, error) {
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
	record := cloneRecord(items[index])
	event.TargetID = record.ID
	event.Resource = resource
	audit, err := s.auditRecordLocked(event, s.nextID+1)
	if err != nil {
		return MutationResult{}, err
	}
	s.nextID++
	s.resources[resource] = append(append([]Record(nil), items[:index]...), items[index+1:]...)
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
	if record.Values == nil {
		record.Values = map[string]string{}
	}
	record.Values[fileDeletionStateField] = fileDeletionPending
	record.Values[fileDeletionRequestedAtField] = s.now().UTC().Format(time.RFC3339)
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

func (s *Store) PurgeTombstonedFileWithAudit(id string, event AuditEvent) (MutationResult, error) {
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
	if !isTombstonedFile(record) {
		return MutationResult{}, ErrInvalidRecord
	}
	event.TargetID = record.ID
	event.Resource = "files"
	audit, err := s.auditRecordLocked(event, s.nextID+1)
	if err != nil {
		return MutationResult{}, err
	}
	s.nextID++
	s.resources["files"] = append(append([]Record(nil), items[:index]...), items[index+1:]...)
	s.resources["audit-logs"] = append(s.resources["audit-logs"], audit)
	if err := s.persistLocked(); err != nil {
		s.restoreSnapshotLocked(previous)
		return MutationResult{}, err
	}
	return MutationResult{Record: record, Audit: cloneRecord(audit)}, nil
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
	return strings.TrimSpace(record.Values[fileDeletionStateField]) == fileDeletionPending
}

func visibleRecords(resource string, records []Record) []Record {
	if resource != "files" {
		return records
	}
	visible := make([]Record, 0, len(records))
	for _, record := range records {
		if !isTombstonedFile(record) {
			visible = append(visible, record)
		}
	}
	return visible
}
