package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type GORMRepository struct {
	db *gorm.DB
}

const (
	mysqlSessionReplacementTable = sessionsTable + "_digest_v2"
	mysqlSessionLegacyTable      = sessionsTable + "_legacy_v1"
)

type gormSessionRecord struct {
	TokenDigest string     `gorm:"primaryKey;size:80;column:token_digest"`
	Username    string     `gorm:"size:256;not null;column:username"`
	IssuedAt    time.Time  `gorm:"not null;column:issued_at"`
	ExpiresAt   time.Time  `gorm:"not null;index;column:expires_at"`
	RevokedAt   *time.Time `gorm:"column:revoked_at"`
}

func (gormSessionRecord) TableName() string {
	return sessionsTable
}

func NewGORMRepository(ctx context.Context, db *gorm.DB) (*GORMRepository, error) {
	quietDB := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	repository := &GORMRepository{db: quietDB}
	if err := repository.ensureSchema(ctx); err != nil {
		return nil, err
	}
	return repository, nil
}

func (r *GORMRepository) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	db, err := r.db.DB()
	if err != nil {
		return err
	}
	return db.Close()
}

func (r *GORMRepository) ensureSchema(ctx context.Context) error {
	db := r.db.WithContext(ctx)
	if db.Dialector.Name() == "mysql" {
		if err := recoverMySQLSessionTableSwap(db); err != nil {
			return err
		}
	}
	migrator := db.Migrator()
	if !migrator.HasTable(&gormSessionRecord{}) {
		if err := db.AutoMigrate(&gormSessionRecord{}); err != nil {
			return err
		}
		return verifyGORMDigestSchema(db)
	}
	legacy := migrator.HasColumn(&gormSessionRecord{}, "token") || !migrator.HasColumn(&gormSessionRecord{}, "token_digest")
	if !legacy {
		if err := db.AutoMigrate(&gormSessionRecord{}); err != nil {
			return err
		}
		return verifyGORMDigestSchema(db)
	}
	if db.Dialector.Name() == "mysql" {
		if err := replaceMySQLSessionTable(db); err != nil {
			return err
		}
	} else {
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Migrator().DropTable(&gormSessionRecord{}); err != nil {
				return err
			}
			return tx.AutoMigrate(&gormSessionRecord{})
		}); err != nil {
			return err
		}
	}
	return verifyGORMDigestSchema(db)
}

func replaceMySQLSessionTable(db *gorm.DB) error {
	migrator := db.Migrator()
	for _, table := range []string{mysqlSessionReplacementTable, mysqlSessionLegacyTable} {
		if migrator.HasTable(table) {
			if err := migrator.DropTable(table); err != nil {
				return err
			}
		}
	}
	if err := db.Table(mysqlSessionReplacementTable).AutoMigrate(&gormSessionRecord{}); err != nil {
		return err
	}
	if !migrator.HasTable(mysqlSessionReplacementTable) {
		return errors.New("session replacement table was not created")
	}
	statement := fmt.Sprintf("RENAME TABLE %s TO %s, %s TO %s", sessionsTable, mysqlSessionLegacyTable, mysqlSessionReplacementTable, sessionsTable)
	if err := db.Exec(statement).Error; err != nil {
		return err
	}
	if err := db.Migrator().DropTable(mysqlSessionLegacyTable); err != nil {
		return err
	}
	return nil
}

func recoverMySQLSessionTableSwap(db *gorm.DB) error {
	migrator := db.Migrator()
	hasCurrent := migrator.HasTable(sessionsTable)
	hasReplacement := migrator.HasTable(mysqlSessionReplacementTable)
	hasLegacy := migrator.HasTable(mysqlSessionLegacyTable)
	if !hasCurrent {
		switch {
		case hasReplacement:
			if err := migrator.RenameTable(mysqlSessionReplacementTable, sessionsTable); err != nil {
				return err
			}
			hasCurrent = true
			hasReplacement = false
		case hasLegacy:
			if err := migrator.RenameTable(mysqlSessionLegacyTable, sessionsTable); err != nil {
				return err
			}
			hasCurrent = true
			hasLegacy = false
		}
	}
	if hasCurrent && hasReplacement {
		if err := migrator.DropTable(mysqlSessionReplacementTable); err != nil {
			return err
		}
	}
	if hasCurrent && hasLegacy {
		if err := migrator.DropTable(mysqlSessionLegacyTable); err != nil {
			return err
		}
	}
	return nil
}

func verifyGORMDigestSchema(db *gorm.DB) error {
	migrator := db.Migrator()
	if !migrator.HasTable(&gormSessionRecord{}) || !migrator.HasColumn(&gormSessionRecord{}, "token_digest") {
		return errors.New("session digest schema is missing token_digest")
	}
	if migrator.HasColumn(&gormSessionRecord{}, "token") {
		return errors.New("session digest schema still contains legacy token column")
	}
	return nil
}

func (r *GORMRepository) Load(ctx context.Context) (Snapshot, error) {
	var records []gormSessionRecord
	if err := r.db.WithContext(ctx).Order("token_digest ASC").Find(&records).Error; err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{Sessions: map[string]StoredSession{}}
	for _, record := range records {
		session := sessionFromGORMRecord(record)
		if err := validateStoredSessionForKey(record.TokenDigest, session); err != nil {
			return Snapshot{}, err
		}
		snapshot.Sessions[record.TokenDigest] = session
	}
	return snapshot, nil
}

func (r *GORMRepository) Create(ctx context.Context, session StoredSession) error {
	if err := validateStoredSessionForKey(session.TokenDigest, session); err != nil {
		return err
	}
	record := gormSessionRecordFromSession(session)
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *GORMRepository) Resolve(ctx context.Context, tokenDigest string, now time.Time) (StoredSession, bool, error) {
	if err := validateTokenDigest(tokenDigest); err != nil {
		return StoredSession{}, false, err
	}
	var record gormSessionRecord
	err := r.activeQuery(r.db.WithContext(ctx), tokenDigest, now).Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return StoredSession{}, false, nil
	}
	if err != nil {
		return StoredSession{}, false, err
	}
	session := sessionFromGORMRecord(record)
	if err := validateStoredSessionForKey(tokenDigest, session); err != nil {
		return StoredSession{}, false, err
	}
	return session, true, nil
}

func (r *GORMRepository) Renew(ctx context.Context, tokenDigest string, now time.Time, expiresAt time.Time) (StoredSession, bool, error) {
	if err := validateTokenDigest(tokenDigest); err != nil {
		return StoredSession{}, false, err
	}
	return r.updateActive(ctx, tokenDigest, now, map[string]any{"expires_at": expiresAt.UTC()}, true)
}

func (r *GORMRepository) Revoke(ctx context.Context, tokenDigest string, now time.Time) (StoredSession, bool, error) {
	if err := validateTokenDigest(tokenDigest); err != nil {
		return StoredSession{}, false, err
	}
	return r.updateActive(ctx, tokenDigest, now, map[string]any{"revoked_at": now.UTC()}, false)
}

func (r *GORMRepository) updateActive(ctx context.Context, tokenDigest string, now time.Time, values map[string]any, acceptActiveNoOp bool) (StoredSession, bool, error) {
	var record gormSessionRecord
	updated := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := r.activeQuery(tx.Model(&gormSessionRecord{}), tokenDigest, now).Updates(values)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			if !acceptActiveNoOp {
				return nil
			}
			err := r.activeQuery(tx, tokenDigest, now).Take(&record).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			updated = true
			return nil
		}
		updated = true
		return tx.Where("token_digest = ?", tokenDigest).Take(&record).Error
	})
	if err != nil {
		return StoredSession{}, false, err
	}
	if !updated {
		return StoredSession{}, false, nil
	}
	session := sessionFromGORMRecord(record)
	if err := validateStoredSessionForKey(tokenDigest, session); err != nil {
		return StoredSession{}, false, err
	}
	return session, true, nil
}

func (r *GORMRepository) activeQuery(db *gorm.DB, tokenDigest string, now time.Time) *gorm.DB {
	return db.Where("token_digest = ? AND revoked_at IS NULL AND expires_at > ?", tokenDigest, now.UTC())
}

func gormSessionRecordFromSession(session StoredSession) gormSessionRecord {
	record := gormSessionRecord{
		TokenDigest: session.TokenDigest,
		Username:    session.Username,
		IssuedAt:    session.IssuedAt.UTC(),
		ExpiresAt:   session.ExpiresAt.UTC(),
	}
	if !session.RevokedAt.IsZero() {
		revokedAt := session.RevokedAt.UTC()
		record.RevokedAt = &revokedAt
	}
	return record
}

func sessionFromGORMRecord(record gormSessionRecord) StoredSession {
	session := StoredSession{
		TokenDigest: record.TokenDigest,
		Username:    record.Username,
		IssuedAt:    record.IssuedAt.UTC(),
		ExpiresAt:   record.ExpiresAt.UTC(),
	}
	if record.RevokedAt != nil {
		session.RevokedAt = record.RevokedAt.UTC()
	}
	return session
}
