package httpapi

import (
	"testing"

	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/dataprotection"
)

func TestNewFailsClosedWhenEncryptedCapabilitiesHaveNoProtectedStore(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("New() did not fail closed for encrypted capabilities without a protected Store")
		}
	}()
	New(ServerOptions{Capabilities: []capability.Manifest{{
		ID: "protected-http-test",
		Admin: capability.AdminSurface{Resources: []capability.AdminResource{{
			Resource: "protected-http-records", Protection: &capability.AdminResourceProtection{SchemaVersion: 1, Scope: "global"},
			Fields: []capability.AdminField{{
				Key: "governmentReference", Source: "values", StorageMode: capability.FieldStorageEncrypted,
				Protection: &capability.AdminFieldProtection{Format: dataprotection.FormatAES256GCMV1, Normalization: dataprotection.NormalizationRawV1},
			}},
		}}},
	}}})
}
