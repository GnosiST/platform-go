package sensitivemigration

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"slices"
	"strings"

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

func EscrowSetHash(entries []EscrowEntry) (string, error) {
	canonical := append([]EscrowEntry(nil), entries...)
	slices.SortFunc(canonical, func(left, right EscrowEntry) int {
		for _, values := range [][2]string{
			{left.RunID, right.RunID}, {left.TenantID, right.TenantID}, {left.Resource, right.Resource},
			{left.RecordID, right.RecordID}, {left.FieldKey, right.FieldKey},
		} {
			if comparison := strings.Compare(values[0], values[1]); comparison != 0 {
				return comparison
			}
		}
		return 0
	})
	hash := sha256.New()
	hash.Write([]byte("platform-go:sensitive-migration:escrow-set:v1"))
	previousCoordinate := ""
	for _, entry := range canonical {
		if entry.RunID == "" || entry.TenantID == "" || entry.Resource == "" || entry.RecordID == "" || entry.FieldKey == "" ||
			entry.ProtectedOriginal == "" || !canonicalSHA256(entry.MigratedValueHash) {
			return "", ErrInvalidOptions
		}
		coordinate := strings.Join([]string{entry.RunID, entry.TenantID, entry.Resource, entry.RecordID, entry.FieldKey}, "\x00")
		if coordinate == previousCoordinate {
			return "", ErrInvalidOptions
		}
		for _, value := range []string{
			entry.RunID, entry.TenantID, entry.Resource, entry.RecordID, entry.FieldKey,
			entry.ProtectedOriginal, entry.MigratedValueHash,
		} {
			writeEscrowHashPart(hash, value)
		}
		previousCoordinate = coordinate
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
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
