package adminresource

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

var (
	ErrUnknownResource = errors.New("unknown admin resource")
	ErrRecordNotFound  = errors.New("admin resource record not found")
	ErrInvalidRecord   = errors.New("invalid admin resource record")
)

type Record struct {
	ID          string            `json:"id"`
	Code        string            `json:"code"`
	Name        string            `json:"name"`
	Status      string            `json:"status"`
	Description string            `json:"description,omitempty"`
	UpdatedAt   string            `json:"updatedAt"`
	Values      map[string]string `json:"values,omitempty"`
}

type WriteInput struct {
	Code        string            `json:"code"`
	Name        string            `json:"name"`
	Status      string            `json:"status"`
	Description string            `json:"description"`
	Values      map[string]string `json:"values"`
}

type Store struct {
	mu        sync.Mutex
	resources map[string][]Record
	nextID    int
	now       func() time.Time
}

func NewStore() *Store {
	store := &Store{
		resources: seedResources(),
		nextID:    1000,
		now:       time.Now,
	}
	return store
}

func (s *Store) List(resource string) ([]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, ok := s.resources[resource]
	if !ok {
		return nil, ErrUnknownResource
	}
	return cloneRecords(items), nil
}

func (s *Store) Create(resource string, input WriteInput) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, ok := s.resources[resource]
	if !ok {
		return Record{}, ErrUnknownResource
	}
	record, err := s.recordFromInput(resource, "", input)
	if err != nil {
		return Record{}, err
	}
	s.nextID++
	record.ID = fmt.Sprintf("%s-%d", resource, s.nextID)
	s.resources[resource] = append(items, record)
	return cloneRecord(record), nil
}

func (s *Store) Update(resource string, id string, input WriteInput) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, ok := s.resources[resource]
	if !ok {
		return Record{}, ErrUnknownResource
	}
	index := slices.IndexFunc(items, func(record Record) bool {
		return record.ID == id
	})
	if index < 0 {
		return Record{}, ErrRecordNotFound
	}
	record, err := s.recordFromInput(resource, id, input)
	if err != nil {
		return Record{}, err
	}
	if record.Code == "" {
		record.Code = items[index].Code
	}
	items[index] = record
	s.resources[resource] = items
	return cloneRecord(record), nil
}

func (s *Store) Delete(resource string, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, ok := s.resources[resource]
	if !ok {
		return ErrUnknownResource
	}
	index := slices.IndexFunc(items, func(record Record) bool {
		return record.ID == id
	})
	if index < 0 {
		return ErrRecordNotFound
	}
	s.resources[resource] = slices.Delete(items, index, index+1)
	return nil
}

func (s *Store) recordFromInput(resource string, id string, input WriteInput) (Record, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Record{}, ErrInvalidRecord
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "enabled"
	}
	code := strings.TrimSpace(input.Code)
	return Record{
		ID:          id,
		Code:        code,
		Name:        name,
		Status:      status,
		Description: strings.TrimSpace(input.Description),
		UpdatedAt:   s.now().UTC().Format(time.RFC3339),
		Values:      cloneValues(input.Values),
	}, nil
}

func cloneRecords(records []Record) []Record {
	cloned := make([]Record, 0, len(records))
	for _, record := range records {
		cloned = append(cloned, cloneRecord(record))
	}
	return cloned
}

func cloneRecord(record Record) Record {
	record.Values = cloneValues(record.Values)
	return record
}

func cloneValues(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func seedResources() map[string][]Record {
	updatedAt := "2026-07-04T00:00:00Z"
	return map[string][]Record{
		"overview": {
			seed("overview-platform", "platform", "Platform Runtime", "healthy", "Platform runtime overview.", updatedAt, map[string]string{"domain": "foundation"}),
		},
		"tenants": {
			seed("tenant-platform", "platform", "Platform Tenant", "enabled", "Default tenant for platform administration.", updatedAt, map[string]string{"isolation": "shared"}),
			seed("tenant-demo", "demo", "Demo Tenant", "enabled", "Reusable tenant for demos and fixtures.", updatedAt, map[string]string{"isolation": "sandbox"}),
		},
		"users": {
			seed("user-admin", "admin", "Platform Admin", "enabled", "Default administrator account.", updatedAt, map[string]string{"role": "super-admin"}),
			seed("user-ops", "ops", "Operations User", "enabled", "Operations account for monitoring tasks.", updatedAt, map[string]string{"role": "operator"}),
		},
		"roles": {
			seed("role-super-admin", "super-admin", "Super Admin", "enabled", "Full platform administration role.", updatedAt, map[string]string{"permissions": "*"}),
			seed("role-platform-admin", "platform-admin", "Platform Admin", "enabled", "Standard platform resource management role.", updatedAt, map[string]string{"permissions": "admin:*"}),
		},
		"menus": {
			seed("menu-capabilities", "capabilities", "Capabilities", "enabled", "Capability management entry.", updatedAt, map[string]string{"route": "/capabilities"}),
			seed("menu-tenants", "tenants", "Tenants", "enabled", "Tenant management entry.", updatedAt, map[string]string{"route": "/tenants"}),
		},
		"api-resources": {
			seed("api-capabilities", "GET:/api/capabilities", "Capability List API", "enabled", "Capability introspection endpoint.", updatedAt, map[string]string{"method": "GET"}),
			seed("api-admin-resources", "GET:/api/admin/resources/:resource", "Admin Resource API", "enabled", "Generic admin resource endpoint.", updatedAt, map[string]string{"method": "GET"}),
		},
		"dictionary-parameters": {
			seed("dict-capability-kind", "capability-kind", "Capability Kind", "enabled", "Core/plugin/optional/disabled enum.", updatedAt, map[string]string{"scope": "platform"}),
			seed("param-brand-name", "brand.name", "Brand Name", "enabled", "Displayed product name.", updatedAt, map[string]string{"scope": "branding"}),
		},
		"audit": {
			seed("audit-bootstrap", "platform.bootstrap", "Platform Bootstrap", "recorded", "Initial platform bootstrap event.", updatedAt, map[string]string{"actor": "system"}),
		},
		"monitoring": {
			seed("monitor-api", "platform-api", "Platform API", "healthy", "Core API process health.", updatedAt, map[string]string{"target": "http"}),
		},
		"settings": {
			seed("setting-branding", "branding", "Branding Settings", "enabled", "Product name, logo and theme settings.", updatedAt, map[string]string{"capability": "branding"}),
		},
	}
}

func seed(id string, code string, name string, status string, description string, updatedAt string, values map[string]string) Record {
	return Record{ID: id, Code: code, Name: name, Status: status, Description: description, UpdatedAt: updatedAt, Values: values}
}
