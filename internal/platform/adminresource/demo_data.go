package adminresource

import (
	"slices"
	"strings"

	"github.com/GnosiST/platform-go/internal/platform/capability"
)

type DemoDataApplyResult struct {
	Resource string
	Applied  int
}

func (s *Store) ApplyDemoDataSet(dataset capability.DemoDataSet) (DemoDataApplyResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.prepareMutationLocked()
	if err != nil {
		return DemoDataApplyResult{}, err
	}
	items, ok := s.resources[dataset.Resource]
	if !ok {
		return DemoDataApplyResult{}, ErrUnknownResource
	}
	applied := 0
	for _, demoRecord := range dataset.Records {
		record, err := s.demoRecordFromDeclaration(dataset.Resource, demoRecord)
		if err != nil {
			s.restoreSnapshotLocked(previous)
			return DemoDataApplyResult{}, err
		}
		index := slices.IndexFunc(items, func(item Record) bool {
			return item.ID == record.ID || (record.Code != "" && item.Code == record.Code)
		})
		if index >= 0 {
			items[index] = record
		} else {
			items = append(items, record)
		}
		applied++
	}
	s.resources[dataset.Resource] = items
	if err := s.persistLocked(); err != nil {
		s.restoreSnapshotLocked(previous)
		return DemoDataApplyResult{}, err
	}
	return DemoDataApplyResult{Resource: dataset.Resource, Applied: applied}, nil
}

func (s *Store) demoRecordFromDeclaration(resource string, record capability.DemoRecord) (Record, error) {
	input := WriteInput{
		Code:        record.Code,
		Name:        record.Name,
		Status:      record.Status,
		Description: record.Description,
		Values:      cloneValues(record.Values),
	}
	genericRecord, err := s.recordFromInputWithOrigin(resource, strings.TrimSpace(record.ID), input, WriteOriginInternal)
	if err != nil {
		return Record{}, err
	}
	if genericRecord.Code == "" {
		genericRecord.Code = strings.TrimSpace(record.Code)
	}
	return genericRecord, nil
}
