package adminresource

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"platform-go/internal/platform/capability"
)

var (
	ErrDeletionPolicyMissing   = errors.New("admin resource deletion policy is missing")
	ErrDeletionDisabled        = errors.New("admin resource deletion is disabled")
	ErrDeletionRequiresAdapter = errors.New("admin resource deletion requires a specialized adapter")
	ErrDeletionCleanupStarted  = errors.New("admin resource external cleanup has started")
	ErrRecordDeleted           = errors.New("admin resource record is deleted")
	ErrRecordNotDeleted        = errors.New("admin resource record is not deleted")
	ErrRecordReferenced        = errors.New("admin resource record is referenced")
	ErrRestoreWindowExpired    = errors.New("admin resource restore window has expired")
	ErrRetentionNotConfigured  = errors.New("admin resource retention is not configured")
	ErrRetentionNotElapsed     = errors.New("admin resource retention has not elapsed")
	ErrRetentionPolicyMismatch = errors.New("admin resource retention policy mismatch")
)

type MaintenanceRetentionPolicy struct {
	Mode          string
	PolicyVersion uint32
	RetentionDays int
	AutoPurge     bool
}

type RecordReference struct {
	Resource string `json:"resource"`
	RecordID string `json:"recordId"`
	Field    string `json:"field"`
}

func isLifecycleDeleted(record Record) bool {
	return strings.TrimSpace(record.DeletedAt) != ""
}

func (s *Store) deletionPolicyLocked(resource string) (ResourceDeletionPolicy, error) {
	schema, ok := s.schemas[resource]
	if !ok {
		return ResourceDeletionPolicy{}, ErrUnknownResource
	}
	if schema.Deletion == nil || strings.TrimSpace(schema.Deletion.Mode) == "" || schema.Deletion.PolicyVersion == 0 {
		return ResourceDeletionPolicy{}, ErrDeletionPolicyMissing
	}
	return *schema.Deletion, nil
}

func (s *Store) deletionMutationLocked(resource string, id string, actor string, reason string) (Record, []Record, error) {
	items, ok := s.resources[resource]
	if !ok {
		return Record{}, nil, ErrUnknownResource
	}
	index := recordIndexByID(items, id)
	if index < 0 {
		return Record{}, nil, ErrRecordNotFound
	}
	policy, err := s.deletionPolicyLocked(resource)
	if err != nil {
		return Record{}, nil, err
	}
	if isLifecycleDeleted(items[index]) {
		if policy.Mode == capability.AdminDeletionSoftDelete {
			return cloneRecord(items[index]), append([]Record(nil), items...), nil
		}
		return Record{}, nil, ErrRecordDeleted
	}
	if policy.RestrictReferences || policy.Mode == capability.AdminDeletionRestrict {
		if references := s.activeReferencesLocked(resource, items[index]); len(references) > 0 {
			return Record{}, nil, fmt.Errorf("%w: %s.%s via %s", ErrRecordReferenced, references[0].Resource, references[0].RecordID, references[0].Field)
		}
	}
	record := cloneRecord(items[index])
	switch policy.Mode {
	case capability.AdminDeletionDisabled, capability.AdminDeletionAppendOnly:
		return Record{}, nil, ErrDeletionDisabled
	case capability.AdminDeletionRevoke, capability.AdminDeletionTombstone:
		return Record{}, nil, ErrDeletionRequiresAdapter
	case capability.AdminDeletionSoftDelete:
		now := s.now().UTC()
		retention, ok := capability.AdminRetentionDuration(policy.RetentionDays)
		if !ok {
			return Record{}, nil, ErrDeletionPolicyMissing
		}
		record.DeletedAt = now.Format(time.RFC3339)
		record.DeletedBy = strings.TrimSpace(actor)
		if record.DeletedBy == "" {
			record.DeletedBy = "system"
		}
		record.DeleteReason = strings.TrimSpace(reason)
		if record.DeleteReason == "" {
			record.DeleteReason = "deleted"
		}
		record.DeletionPolicyVersion = policy.PolicyVersion
		if policy.RetentionDays > 0 {
			record.PurgeAfter = now.Add(retention).Format(time.RFC3339)
		}
		record.UpdatedAt = now.Format(time.RFC3339)
		nextItems := append([]Record(nil), items...)
		nextItems[index] = record
		return record, nextItems, nil
	case capability.AdminDeletionRestrict, capability.AdminDeletionHardDelete:
		nextItems := slices.Delete(append([]Record(nil), items...), index, index+1)
		return record, nextItems, nil
	default:
		return Record{}, nil, ErrDeletionPolicyMissing
	}
}

func (s *Store) restoreMutationLocked(resource string, id string) (Record, []Record, error) {
	items, ok := s.resources[resource]
	if !ok {
		return Record{}, nil, ErrUnknownResource
	}
	index := recordIndexByID(items, id)
	if index < 0 {
		return Record{}, nil, ErrRecordNotFound
	}
	policy, err := s.deletionPolicyLocked(resource)
	if err != nil {
		return Record{}, nil, err
	}
	if policy.Mode != capability.AdminDeletionSoftDelete && policy.Mode != capability.AdminDeletionTombstone {
		return Record{}, nil, ErrDeletionDisabled
	}
	record := cloneRecord(items[index])
	if !isLifecycleDeleted(record) && !isTombstonedFile(record) {
		return Record{}, nil, ErrRecordNotDeleted
	}
	if resource == "files" && fileDeletionState(record) != fileDeletionPending {
		return Record{}, nil, ErrDeletionCleanupStarted
	}
	if strings.TrimSpace(record.PurgeAfter) != "" {
		purgeAfter, err := time.Parse(time.RFC3339, record.PurgeAfter)
		if err != nil {
			return Record{}, nil, fmt.Errorf("%w: invalid purgeAfter", ErrInvalidRecord)
		}
		if !s.now().UTC().Before(purgeAfter.UTC()) {
			return Record{}, nil, ErrRestoreWindowExpired
		}
	}
	record.DeletedAt = ""
	record.DeletedBy = ""
	record.DeleteReason = ""
	record.PurgeAfter = ""
	record.DeletionPolicyVersion = 0
	record.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	if record.Values != nil {
		delete(record.Values, fileDeletionStateField)
		delete(record.Values, fileDeletionRequestedAtField)
	}
	nextItems := append([]Record(nil), items...)
	nextItems[index] = record
	return record, nextItems, nil
}

func (s *Store) purgeMutationLocked(resource string, id string) (Record, []Record, error) {
	items, ok := s.resources[resource]
	if !ok {
		return Record{}, nil, ErrUnknownResource
	}
	index := recordIndexByID(items, id)
	if index < 0 {
		return Record{}, nil, ErrRecordNotFound
	}
	record := cloneRecord(items[index])
	if !isLifecycleDeleted(record) && !isTombstonedFile(record) {
		return Record{}, nil, ErrRecordNotDeleted
	}
	if strings.TrimSpace(record.PurgeAfter) == "" {
		return Record{}, nil, ErrRetentionNotConfigured
	}
	purgeAfter, err := time.Parse(time.RFC3339, record.PurgeAfter)
	if err != nil {
		return Record{}, nil, fmt.Errorf("%w: invalid purgeAfter", ErrInvalidRecord)
	}
	if s.now().UTC().Before(purgeAfter.UTC()) {
		return Record{}, nil, ErrRetentionNotElapsed
	}
	return record, slices.Delete(append([]Record(nil), items...), index, index+1), nil
}

func (s *Store) purgeMutationWithPolicyLocked(resource string, id string, policy MaintenanceRetentionPolicy) (Record, []Record, error) {
	items, ok := s.resources[resource]
	if !ok {
		return Record{}, nil, ErrUnknownResource
	}
	index := recordIndexByID(items, id)
	if index < 0 {
		return Record{}, nil, ErrRecordNotFound
	}
	record := cloneRecord(items[index])
	if !isLifecycleDeleted(record) && !isTombstonedFile(record) {
		return Record{}, nil, ErrRecordNotDeleted
	}
	if err := s.requireMaintenanceRetentionElapsedLocked(resource, record, policy); err != nil {
		return Record{}, nil, err
	}
	return record, slices.Delete(append([]Record(nil), items...), index, index+1), nil
}

func (s *Store) requireMaintenanceRetentionElapsedLocked(resource string, record Record, requested MaintenanceRetentionPolicy) error {
	configured, err := s.deletionPolicyLocked(resource)
	if err != nil {
		return err
	}
	retention, ok := capability.AdminRetentionDuration(requested.RetentionDays)
	if !configured.AutoPurge || !requested.AutoPurge || requested.PolicyVersion == 0 || requested.RetentionDays <= 0 ||
		!ok || retention <= 0 || configured.Mode != requested.Mode || !capability.SupportsAdminAutoPurge(resource, requested.Mode) ||
		configured.PolicyVersion != requested.PolicyVersion || configured.RetentionDays != requested.RetentionDays || configured.AutoPurge != requested.AutoPurge {
		return ErrRetentionPolicyMismatch
	}
	deletedAt, err := time.Parse(time.RFC3339, record.DeletedAt)
	if err != nil {
		return fmt.Errorf("%w: invalid deletedAt", ErrInvalidRecord)
	}
	eligibleAt := deletedAt.UTC().Add(retention)
	if s.now().UTC().Before(eligibleAt) {
		return ErrRetentionNotElapsed
	}
	return nil
}

func (s *Store) activeReferencesLocked(targetResource string, target Record) []RecordReference {
	references := make([]RecordReference, 0)
	for sourceResource, schema := range s.schemas {
		for _, field := range schema.Fields {
			if field.Relation == nil || field.Relation.Resource != targetResource {
				continue
			}
			targetValue := strings.TrimSpace(recordValue(target, field.Relation.ValueField))
			if targetValue == "" {
				continue
			}
			for _, source := range s.resources[sourceResource] {
				if sourceResource == targetResource && source.ID == target.ID {
					continue
				}
				values := []string{recordValue(source, field.Key)}
				if field.Relation.Multiple || field.Type == "multiselect" {
					values = splitQueryList(values[0])
				}
				if slices.ContainsFunc(values, func(value string) bool { return strings.EqualFold(strings.TrimSpace(value), targetValue) }) {
					references = append(references, RecordReference{Resource: sourceResource, RecordID: source.ID, Field: field.Key})
				}
			}
		}
	}
	return references
}
