package sensitivemigration

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"slices"
	"strings"

	"github.com/GnosiST/platform-go/internal/platform/dataprotection"
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
		return strings.Compare(EscrowCursorKey(EscrowCursorFromEntry(left)), EscrowCursorKey(EscrowCursorFromEntry(right)))
	})
	hasher := NewEscrowSetHasher()
	if err := hasher.Add(canonical...); err != nil {
		return "", err
	}
	value, _, err := hasher.Sum()
	return value, err
}

type EscrowSetHasher struct {
	hash     hash.Hash
	previous EscrowCursor
	count    int
	failed   bool
}

func NewEscrowSetHasher() *EscrowSetHasher {
	digest := sha256.New()
	_, _ = digest.Write([]byte("platform-go:sensitive-migration:escrow-set:v1"))
	return &EscrowSetHasher{hash: digest}
}

func (h *EscrowSetHasher) Add(entries ...EscrowEntry) error {
	if h == nil || h.hash == nil || h.failed {
		return ErrInvalidOptions
	}
	for _, entry := range entries {
		cursor := EscrowCursorFromEntry(entry)
		if entry.RunID == "" || entry.TenantID == "" || entry.Resource == "" || entry.RecordID == "" || entry.FieldKey == "" ||
			entry.ProtectedOriginal == "" || !canonicalSHA256(entry.MigratedValueHash) || h.count > 0 && compareEscrowCursor(cursor, h.previous) <= 0 {
			h.failed = true
			return ErrInvalidOptions
		}
		for _, value := range []string{
			entry.RunID, entry.TenantID, entry.Resource, entry.RecordID, entry.FieldKey,
			entry.ProtectedOriginal, entry.MigratedValueHash,
		} {
			writeEscrowHashPart(h.hash, value)
		}
		h.previous = cursor
		h.count++
	}
	return nil
}

func (h *EscrowSetHasher) Sum() (string, int, error) {
	if h == nil || h.hash == nil || h.failed {
		return "", 0, ErrInvalidOptions
	}
	return "sha256:" + hex.EncodeToString(h.hash.Sum(nil)), h.count, nil
}

func EscrowCursorFromEntry(entry EscrowEntry) EscrowCursor {
	return EscrowCursor{RunID: entry.RunID, TenantID: entry.TenantID, Resource: entry.Resource, RecordID: entry.RecordID, FieldKey: entry.FieldKey}
}

func compareEscrowCursor(left, right EscrowCursor) int {
	return strings.Compare(EscrowCursorKey(left), EscrowCursorKey(right))
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
