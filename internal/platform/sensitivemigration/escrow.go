package sensitivemigration

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"

	"platform-go/internal/platform/dataprotection"
)

const escrowResource = "migration-rollback"

func EscrowContext(runID, tenantID, resource, recordID, fieldKey string) (dataprotection.FieldPolicy, dataprotection.FieldContext) {
	policy := dataprotection.FieldPolicy{
		Format: dataprotection.FormatAES256GCMV1, Normalization: dataprotection.NormalizationRawV1,
	}
	return policy, dataprotection.FieldContext{
		TenantID: tenantID, Resource: escrowResource,
		RecordID: runID + ":" + escrowCoordinateHash(resource, recordID, fieldKey),
		FieldKey: "original-value", SchemaVersion: 1,
	}
}

func HashMigratedValue(value string) string {
	hash := sha256.New()
	hash.Write([]byte("platform-go:sensitive-migration:migrated-value:v1"))
	writeEscrowHashPart(hash, value)
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

func escrowCoordinateHash(values ...string) string {
	hash := sha256.New()
	hash.Write([]byte("platform-go:sensitive-migration:escrow-coordinate:v1"))
	for _, value := range values {
		writeEscrowHashPart(hash, value)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

type hashWriter interface {
	Write([]byte) (int, error)
}

func writeEscrowHashPart(hash hashWriter, value string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = hash.Write(length[:])
	_, _ = hash.Write([]byte(value))
}
