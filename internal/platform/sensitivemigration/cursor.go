package sensitivemigration

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
)

func TenantCursor(tenantID string) string {
	return stableCursor("tenant", tenantID)
}

func RecordCursor(recordID string) string {
	return stableCursor("record", recordID)
}

func EscrowCursorKey(cursor EscrowCursor) string {
	return stableCursor("escrow", cursor.RunID, cursor.TenantID, cursor.Resource, cursor.RecordID, cursor.FieldKey)
}

func stableCursor(domain string, values ...string) string {
	digest := sha256.New()
	for _, value := range append([]string{"platform-go:sensitive-migration:cursor:v1", domain}, values...) {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		_, _ = digest.Write(size[:])
		_, _ = digest.Write([]byte(value))
	}
	return hex.EncodeToString(digest.Sum(nil))
}
