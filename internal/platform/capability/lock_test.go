package capability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadLockFileNormalizesCapabilities(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "platform-capabilities.lock.json")
	if err := os.WriteFile(lockPath, []byte(`{"version":1,"capabilities":["tenant"," identity ","audit"]}`), 0o600); err != nil {
		t.Fatalf("write lock file: %v", err)
	}

	lock, err := LoadLockFile(lockPath)
	if err != nil {
		t.Fatalf("LoadLockFile() error = %v", err)
	}
	want := []ID{"tenant", "identity", "audit"}
	if len(lock.Capabilities) != len(want) {
		t.Fatalf("capabilities = %#v, want %#v", lock.Capabilities, want)
	}
	for index := range want {
		if lock.Capabilities[index] != want[index] {
			t.Fatalf("capabilities = %#v, want %#v", lock.Capabilities, want)
		}
	}
}

func TestLoadLockFileRejectsInvalidContract(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "version", body: `{"version":2,"capabilities":["tenant"]}`, want: "version must be 1"},
		{name: "empty", body: `{"version":1,"capabilities":[]}`, want: "enabled capabilities must not be empty"},
		{name: "duplicate", body: `{"version":1,"capabilities":["tenant","tenant"]}`, want: `duplicate enabled capability "tenant"`},
		{name: "invalid id", body: `{"version":1,"capabilities":["Tenant"]}`, want: `enabled capability "Tenant" must use lowercase letters, numbers, and hyphens`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lockPath := filepath.Join(t.TempDir(), "platform-capabilities.lock.json")
			if err := os.WriteFile(lockPath, []byte(tt.body), 0o600); err != nil {
				t.Fatalf("write lock file: %v", err)
			}

			_, err := LoadLockFile(lockPath)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("LoadLockFile() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}
