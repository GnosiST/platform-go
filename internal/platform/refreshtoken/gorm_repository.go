package refreshtoken

import (
	"context"
	"sort"
	"time"

	"gorm.io/gorm"
)

const familiesTable = "platform_refresh_token_families"

type GORMRepository struct {
	db *gorm.DB
}

type gormRefreshTokenRecord struct {
	FamilyID          string     `gorm:"size:256;not null;index;column:family_id"`
	TokenID           string     `gorm:"primaryKey;size:256;column:token_id"`
	ParentTokenID     string     `gorm:"size:256;column:parent_token_id"`
	SessionID         string     `gorm:"size:256;not null;index;column:session_id"`
	Username          string     `gorm:"size:256;not null;index;column:username"`
	TenantID          string     `gorm:"size:256;not null;index;column:tenant_id"`
	TokenType         string     `gorm:"size:64;not null;column:token_type"`
	IssuedAt          time.Time  `gorm:"not null;column:issued_at"`
	ExpiresAt         time.Time  `gorm:"not null;index;column:expires_at"`
	RotatedAt         *time.Time `gorm:"column:rotated_at"`
	RevokedAt         *time.Time `gorm:"column:revoked_at"`
	ReusedAt          *time.Time `gorm:"column:reused_at"`
	ReplacedByTokenID string     `gorm:"size:256;column:replaced_by_token_id"`
	TokenHash         string     `gorm:"size:256;not null;uniqueIndex;column:token_hash"`
}

func (gormRefreshTokenRecord) TableName() string {
	return familiesTable
}

func NewGORMRepository(ctx context.Context, db *gorm.DB) (*GORMRepository, error) {
	repository := &GORMRepository{db: db}
	if err := db.WithContext(ctx).AutoMigrate(&gormRefreshTokenRecord{}); err != nil {
		return nil, err
	}
	return repository, nil
}

func (r *GORMRepository) Load(ctx context.Context) (Snapshot, error) {
	var records []gormRefreshTokenRecord
	if err := r.db.WithContext(ctx).Order("family_id ASC, issued_at ASC, token_id ASC").Find(&records).Error; err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{Records: map[string]Record{}}
	for _, record := range records {
		snapshot.Records[record.TokenID] = recordFromGORM(record)
	}
	return snapshot, nil
}

func (r *GORMRepository) Save(ctx context.Context, snapshot Snapshot) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&gormRefreshTokenRecord{}).Error; err != nil {
			return err
		}
		if len(snapshot.Records) == 0 {
			return nil
		}
		tokenIDs := make([]string, 0, len(snapshot.Records))
		for tokenID := range snapshot.Records {
			tokenIDs = append(tokenIDs, tokenID)
		}
		sort.Strings(tokenIDs)
		records := make([]gormRefreshTokenRecord, 0, len(tokenIDs))
		for _, tokenID := range tokenIDs {
			records = append(records, recordToGORM(snapshot.Records[tokenID]))
		}
		return tx.Create(&records).Error
	})
}

func recordFromGORM(record gormRefreshTokenRecord) Record {
	item := Record{
		FamilyID:          record.FamilyID,
		TokenID:           record.TokenID,
		ParentTokenID:     record.ParentTokenID,
		SessionID:         record.SessionID,
		Username:          record.Username,
		TenantID:          record.TenantID,
		TokenType:         record.TokenType,
		IssuedAt:          record.IssuedAt.UTC(),
		ExpiresAt:         record.ExpiresAt.UTC(),
		ReplacedByTokenID: record.ReplacedByTokenID,
		TokenHash:         record.TokenHash,
	}
	if record.RotatedAt != nil {
		item.RotatedAt = record.RotatedAt.UTC()
	}
	if record.RevokedAt != nil {
		item.RevokedAt = record.RevokedAt.UTC()
	}
	if record.ReusedAt != nil {
		item.ReusedAt = record.ReusedAt.UTC()
	}
	return item
}

func recordToGORM(record Record) gormRefreshTokenRecord {
	item := gormRefreshTokenRecord{
		FamilyID:          record.FamilyID,
		TokenID:           record.TokenID,
		ParentTokenID:     record.ParentTokenID,
		SessionID:         record.SessionID,
		Username:          record.Username,
		TenantID:          record.TenantID,
		TokenType:         record.TokenType,
		IssuedAt:          record.IssuedAt.UTC(),
		ExpiresAt:         record.ExpiresAt.UTC(),
		ReplacedByTokenID: record.ReplacedByTokenID,
		TokenHash:         record.TokenHash,
	}
	if !record.RotatedAt.IsZero() {
		rotatedAt := record.RotatedAt.UTC()
		item.RotatedAt = &rotatedAt
	}
	if !record.RevokedAt.IsZero() {
		revokedAt := record.RevokedAt.UTC()
		item.RevokedAt = &revokedAt
	}
	if !record.ReusedAt.IsZero() {
		reusedAt := record.ReusedAt.UTC()
		item.ReusedAt = &reusedAt
	}
	return item
}
