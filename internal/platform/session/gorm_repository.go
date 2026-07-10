package session

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type GORMRepository struct {
	db *gorm.DB
}

type gormSessionRecord struct {
	Token     string     `gorm:"primaryKey;size:256;column:token"`
	Username  string     `gorm:"size:256;not null;column:username"`
	IssuedAt  time.Time  `gorm:"not null;column:issued_at"`
	ExpiresAt time.Time  `gorm:"not null;index;column:expires_at"`
	RevokedAt *time.Time `gorm:"column:revoked_at"`
}

func (gormSessionRecord) TableName() string {
	return sessionsTable
}

func NewGORMRepository(ctx context.Context, db *gorm.DB) (*GORMRepository, error) {
	repository := &GORMRepository{db: db}
	if err := db.WithContext(ctx).AutoMigrate(&gormSessionRecord{}); err != nil {
		return nil, err
	}
	return repository, nil
}

func (r *GORMRepository) Load(ctx context.Context) (Snapshot, error) {
	var records []gormSessionRecord
	if err := r.db.WithContext(ctx).Order("token ASC").Find(&records).Error; err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{Sessions: map[string]Session{}}
	for _, record := range records {
		session := sessionFromGORMRecord(record)
		snapshot.Sessions[record.Token] = session
	}
	return snapshot, nil
}

func (r *GORMRepository) Create(ctx context.Context, session Session) error {
	record := gormSessionRecordFromSession(session)
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *GORMRepository) Resolve(ctx context.Context, token string, now time.Time) (Session, bool, error) {
	var record gormSessionRecord
	err := r.activeQuery(r.db.WithContext(ctx), token, now).Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Session{}, false, nil
	}
	if err != nil {
		return Session{}, false, err
	}
	return sessionFromGORMRecord(record), true, nil
}

func (r *GORMRepository) Renew(ctx context.Context, token string, now time.Time, expiresAt time.Time) (Session, bool, error) {
	return r.updateActive(ctx, token, now, map[string]any{"expires_at": expiresAt.UTC()}, true)
}

func (r *GORMRepository) Revoke(ctx context.Context, token string, now time.Time) (Session, bool, error) {
	return r.updateActive(ctx, token, now, map[string]any{"revoked_at": now.UTC()}, false)
}

func (r *GORMRepository) updateActive(ctx context.Context, token string, now time.Time, values map[string]any, acceptActiveNoOp bool) (Session, bool, error) {
	var record gormSessionRecord
	updated := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := r.activeQuery(tx.Model(&gormSessionRecord{}), token, now).Updates(values)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			if !acceptActiveNoOp {
				return nil
			}
			err := r.activeQuery(tx, token, now).Take(&record).Error
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
		return tx.Where("token = ?", token).Take(&record).Error
	})
	if err != nil {
		return Session{}, false, err
	}
	if !updated {
		return Session{}, false, nil
	}
	return sessionFromGORMRecord(record), true, nil
}

func (r *GORMRepository) activeQuery(db *gorm.DB, token string, now time.Time) *gorm.DB {
	return db.Where("token = ? AND revoked_at IS NULL AND expires_at > ?", token, now.UTC())
}

func gormSessionRecordFromSession(session Session) gormSessionRecord {
	record := gormSessionRecord{
		Token:     session.Token,
		Username:  session.Username,
		IssuedAt:  session.IssuedAt.UTC(),
		ExpiresAt: session.ExpiresAt.UTC(),
	}
	if !session.RevokedAt.IsZero() {
		revokedAt := session.RevokedAt.UTC()
		record.RevokedAt = &revokedAt
	}
	return record
}

func sessionFromGORMRecord(record gormSessionRecord) Session {
	session := Session{
		Token:     record.Token,
		Username:  record.Username,
		IssuedAt:  record.IssuedAt.UTC(),
		ExpiresAt: record.ExpiresAt.UTC(),
	}
	if record.RevokedAt != nil {
		session.RevokedAt = record.RevokedAt.UTC()
	}
	return session
}
