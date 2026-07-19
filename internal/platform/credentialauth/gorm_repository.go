package credentialauth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	authIdentifiersTable      = "auth_identifiers"
	passwordCredentialsTable  = "password_credentials"
	credentialChallengesTable = "credential_challenges"
	smsOTPChallengesTable     = "sms_otp_challenges"
)

type GORMRepository struct {
	db *gorm.DB
}

type gormAuthIdentifier struct {
	PrincipalType    string    `gorm:"column:principal_type;size:32;not null;index"`
	PrincipalID      string    `gorm:"column:principal_id;size:191;not null;index"`
	IdentifierType   string    `gorm:"column:identifier_type;size:32;primaryKey"`
	IdentifierHash   string    `gorm:"column:identifier_hash;size:191;primaryKey"`
	MaskedIdentifier string    `gorm:"column:masked_identifier;size:191;not null"`
	VerifiedAt       time.Time `gorm:"column:verified_at;not null"`
	Status           string    `gorm:"column:status;size:32;not null;index"`
}

type gormPasswordCredential struct {
	PrincipalType     string     `gorm:"column:principal_type;size:32;primaryKey"`
	PrincipalID       string     `gorm:"column:principal_id;size:191;primaryKey"`
	PasswordHash      string     `gorm:"column:password_hash;size:512;not null"`
	Algorithm         string     `gorm:"column:algorithm;size:64;not null"`
	ParamsVersion     string     `gorm:"column:params_version;size:64;not null"`
	PasswordUpdatedAt time.Time  `gorm:"column:password_updated_at;not null"`
	MustChange        bool       `gorm:"column:must_change;not null"`
	FailedAttempts    int        `gorm:"column:failed_attempts;not null"`
	LockedUntil       *time.Time `gorm:"column:locked_until;index"`
	Status            string     `gorm:"column:status;size:32;not null;index"`
}

type gormCredentialChallenge struct {
	ChallengeID           string     `gorm:"column:challenge_id;size:80;primaryKey"`
	Kind                  string     `gorm:"column:kind;size:32;not null"`
	Purpose               string     `gorm:"column:purpose;size:64;not null;index"`
	AnswerDigest          string     `gorm:"column:answer_digest;size:191;not null"`
	ExpiresAt             time.Time  `gorm:"column:expires_at;not null;index"`
	Attempts              int        `gorm:"column:attempts;not null"`
	UsedAt                *time.Time `gorm:"column:used_at;index"`
	ClientFingerprintHash string     `gorm:"column:client_fingerprint_hash;size:191"`
	Status                string     `gorm:"column:status;size:32;not null;index"`
}

type gormSMSOTPChallenge struct {
	ChallengeID string     `gorm:"column:challenge_id;size:80;primaryKey"`
	PhoneHash   string     `gorm:"column:phone_hash;size:191;not null;index"`
	CodeDigest  string     `gorm:"column:code_digest;size:191;not null"`
	ExpiresAt   time.Time  `gorm:"column:expires_at;not null;index"`
	Attempts    int        `gorm:"column:attempts;not null"`
	MessageID   string     `gorm:"column:message_id;size:191"`
	UsedAt      *time.Time `gorm:"column:used_at;index"`
	Status      string     `gorm:"column:status;size:32;not null;index"`
}

func (gormAuthIdentifier) TableName() string      { return authIdentifiersTable }
func (gormPasswordCredential) TableName() string  { return passwordCredentialsTable }
func (gormCredentialChallenge) TableName() string { return credentialChallengesTable }
func (gormSMSOTPChallenge) TableName() string     { return smsOTPChallengesTable }

func NewGORMRepository(ctx context.Context, db *gorm.DB) (*GORMRepository, error) {
	if ctx == nil || db == nil {
		return nil, fmt.Errorf("%w: gorm repository is unavailable", ErrInvalidInput)
	}
	if err := db.WithContext(ctx).AutoMigrate(gormCredentialModels()...); err != nil {
		return nil, fmt.Errorf("%w: migrate credential-auth repository", ErrRepositoryInvariant)
	}
	return OpenGORMRepository(ctx, db)
}

func OpenGORMRepository(ctx context.Context, db *gorm.DB) (*GORMRepository, error) {
	if ctx == nil || db == nil {
		return nil, fmt.Errorf("%w: gorm repository is unavailable", ErrInvalidInput)
	}
	migrator := db.WithContext(ctx).Migrator()
	for _, model := range gormCredentialModels() {
		if !migrator.HasTable(model) {
			return nil, fmt.Errorf("%w: credential-auth gorm schema is missing", ErrRepositoryInvariant)
		}
	}
	return &GORMRepository{db: db}, nil
}

func (r *GORMRepository) Persistent() bool {
	return r != nil && r.db != nil
}

func (r *GORMRepository) UpsertIdentifier(ctx context.Context, record IdentifierRecord) error {
	if !r.ready(ctx) {
		return fmt.Errorf("%w: repository is unavailable", ErrRepositoryInvariant)
	}
	record, err := normalizedIdentifierRecord(record)
	if err != nil {
		return err
	}
	row := gormIdentifierFromRecord(record)
	return r.db.WithContext(ctx).Save(&row).Error
}

func (r *GORMRepository) LookupIdentifier(ctx context.Context, identifierType IdentifierType, identifierHash string) (IdentifierRecord, bool, error) {
	if !r.ready(ctx) {
		return IdentifierRecord{}, false, fmt.Errorf("%w: repository is unavailable", ErrRepositoryInvariant)
	}
	identifierType = IdentifierType(strings.TrimSpace(string(identifierType)))
	identifierHash = strings.TrimSpace(identifierHash)
	if !validIdentifierType(identifierType) || identifierHash == "" {
		return IdentifierRecord{}, false, fmt.Errorf("%w: identifier lookup key is invalid", ErrInvalidInput)
	}
	var row gormAuthIdentifier
	err := r.db.WithContext(ctx).
		Where("identifier_type = ? AND identifier_hash = ?", string(identifierType), identifierHash).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return IdentifierRecord{}, false, nil
	}
	if err != nil {
		return IdentifierRecord{}, false, err
	}
	record := identifierRecordFromGORM(row)
	return record, true, nil
}

func (r *GORMRepository) UpsertPasswordCredential(ctx context.Context, credential PasswordCredential) error {
	if !r.ready(ctx) {
		return fmt.Errorf("%w: repository is unavailable", ErrRepositoryInvariant)
	}
	credential, err := normalizedPasswordCredential(credential)
	if err != nil {
		return err
	}
	row := gormPasswordFromCredential(credential)
	return r.db.WithContext(ctx).Save(&row).Error
}

func (r *GORMRepository) PasswordCredential(ctx context.Context, principal PrincipalRef) (PasswordCredential, bool, error) {
	if !r.ready(ctx) {
		return PasswordCredential{}, false, fmt.Errorf("%w: repository is unavailable", ErrRepositoryInvariant)
	}
	principal, err := principal.normalized()
	if err != nil {
		return PasswordCredential{}, false, err
	}
	var row gormPasswordCredential
	err = r.db.WithContext(ctx).
		Where("principal_type = ? AND principal_id = ?", string(principal.Type), principal.ID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PasswordCredential{}, false, nil
	}
	if err != nil {
		return PasswordCredential{}, false, err
	}
	return passwordCredentialFromGORM(row), true, nil
}

func (r *GORMRepository) UpsertCredentialChallenge(ctx context.Context, challenge CredentialChallenge) error {
	if !r.ready(ctx) {
		return fmt.Errorf("%w: repository is unavailable", ErrRepositoryInvariant)
	}
	challenge, err := normalizedCredentialChallenge(challenge)
	if err != nil {
		return err
	}
	row := gormChallengeFromRecord(challenge)
	return r.db.WithContext(ctx).Save(&row).Error
}

func (r *GORMRepository) CredentialChallenge(ctx context.Context, challengeID string) (CredentialChallenge, bool, error) {
	if !r.ready(ctx) {
		return CredentialChallenge{}, false, fmt.Errorf("%w: repository is unavailable", ErrRepositoryInvariant)
	}
	challengeID = strings.TrimSpace(challengeID)
	if challengeID == "" {
		return CredentialChallenge{}, false, fmt.Errorf("%w: challenge id is required", ErrInvalidInput)
	}
	var row gormCredentialChallenge
	err := r.db.WithContext(ctx).Where("challenge_id = ?", challengeID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CredentialChallenge{}, false, nil
	}
	if err != nil {
		return CredentialChallenge{}, false, err
	}
	return challengeFromGORM(row), true, nil
}

func (r *GORMRepository) UpsertSMSOTPChallenge(ctx context.Context, challenge SMSOTPChallenge) error {
	if !r.ready(ctx) {
		return fmt.Errorf("%w: repository is unavailable", ErrRepositoryInvariant)
	}
	challenge, err := normalizedSMSOTPChallenge(challenge)
	if err != nil {
		return err
	}
	row := gormSMSOTPFromRecord(challenge)
	return r.db.WithContext(ctx).Save(&row).Error
}

func (r *GORMRepository) SMSOTPChallenge(ctx context.Context, challengeID string) (SMSOTPChallenge, bool, error) {
	if !r.ready(ctx) {
		return SMSOTPChallenge{}, false, fmt.Errorf("%w: repository is unavailable", ErrRepositoryInvariant)
	}
	challengeID = strings.TrimSpace(challengeID)
	if challengeID == "" {
		return SMSOTPChallenge{}, false, fmt.Errorf("%w: sms otp challenge id is required", ErrInvalidInput)
	}
	var row gormSMSOTPChallenge
	err := r.db.WithContext(ctx).Where("challenge_id = ?", challengeID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SMSOTPChallenge{}, false, nil
	}
	if err != nil {
		return SMSOTPChallenge{}, false, err
	}
	return smsOTPFromGORM(row), true, nil
}

func (r *GORMRepository) ready(ctx context.Context) bool {
	return r != nil && r.db != nil && checkContext(ctx) == nil
}

func gormCredentialModels() []any {
	return []any{&gormAuthIdentifier{}, &gormPasswordCredential{}, &gormCredentialChallenge{}, &gormSMSOTPChallenge{}}
}

func gormIdentifierFromRecord(record IdentifierRecord) gormAuthIdentifier {
	return gormAuthIdentifier{
		PrincipalType:    string(record.Principal.Type),
		PrincipalID:      record.Principal.ID,
		IdentifierType:   string(record.IdentifierType),
		IdentifierHash:   record.IdentifierHash,
		MaskedIdentifier: record.MaskedIdentifier,
		VerifiedAt:       record.VerifiedAt.UTC(),
		Status:           string(normalizeStatus(record.Status)),
	}
}

func identifierRecordFromGORM(row gormAuthIdentifier) IdentifierRecord {
	return IdentifierRecord{
		Principal:        PrincipalRef{Type: PrincipalType(row.PrincipalType), ID: row.PrincipalID},
		IdentifierType:   IdentifierType(row.IdentifierType),
		IdentifierHash:   row.IdentifierHash,
		MaskedIdentifier: row.MaskedIdentifier,
		VerifiedAt:       row.VerifiedAt.UTC(),
		Status:           RecordStatus(row.Status),
	}
}

func gormPasswordFromCredential(credential PasswordCredential) gormPasswordCredential {
	return gormPasswordCredential{
		PrincipalType:     string(credential.Principal.Type),
		PrincipalID:       credential.Principal.ID,
		PasswordHash:      credential.PasswordHash,
		Algorithm:         credential.Algorithm,
		ParamsVersion:     credential.ParamsVersion,
		PasswordUpdatedAt: credential.PasswordUpdatedAt.UTC(),
		MustChange:        credential.MustChange,
		FailedAttempts:    credential.FailedAttempts,
		LockedUntil:       timePointer(credential.LockedUntil),
		Status:            string(normalizeStatus(credential.Status)),
	}
}

func passwordCredentialFromGORM(row gormPasswordCredential) PasswordCredential {
	return PasswordCredential{
		Principal:         PrincipalRef{Type: PrincipalType(row.PrincipalType), ID: row.PrincipalID},
		PasswordHash:      row.PasswordHash,
		Algorithm:         row.Algorithm,
		ParamsVersion:     row.ParamsVersion,
		PasswordUpdatedAt: row.PasswordUpdatedAt.UTC(),
		MustChange:        row.MustChange,
		FailedAttempts:    row.FailedAttempts,
		LockedUntil:       timeFromPointer(row.LockedUntil),
		Status:            RecordStatus(row.Status),
	}
}

func gormChallengeFromRecord(challenge CredentialChallenge) gormCredentialChallenge {
	return gormCredentialChallenge{
		ChallengeID:           challenge.ChallengeID,
		Kind:                  string(challenge.Kind),
		Purpose:               string(challenge.Purpose),
		AnswerDigest:          challenge.AnswerDigest,
		ExpiresAt:             challenge.ExpiresAt.UTC(),
		Attempts:              challenge.Attempts,
		UsedAt:                timePointer(challenge.UsedAt),
		ClientFingerprintHash: challenge.ClientFingerprintHash,
		Status:                string(normalizeStatus(challenge.Status)),
	}
}

func challengeFromGORM(row gormCredentialChallenge) CredentialChallenge {
	return CredentialChallenge{
		ChallengeID:           row.ChallengeID,
		Kind:                  ChallengeKind(row.Kind),
		Purpose:               ChallengePurpose(row.Purpose),
		AnswerDigest:          row.AnswerDigest,
		ExpiresAt:             row.ExpiresAt.UTC(),
		Attempts:              row.Attempts,
		UsedAt:                timeFromPointer(row.UsedAt),
		ClientFingerprintHash: row.ClientFingerprintHash,
		Status:                RecordStatus(row.Status),
	}
}

func gormSMSOTPFromRecord(challenge SMSOTPChallenge) gormSMSOTPChallenge {
	return gormSMSOTPChallenge{
		ChallengeID: challenge.ChallengeID,
		PhoneHash:   challenge.PhoneHash,
		CodeDigest:  challenge.CodeDigest,
		ExpiresAt:   challenge.ExpiresAt.UTC(),
		Attempts:    challenge.Attempts,
		MessageID:   challenge.MessageID,
		UsedAt:      timePointer(challenge.UsedAt),
		Status:      string(normalizeStatus(challenge.Status)),
	}
}

func smsOTPFromGORM(row gormSMSOTPChallenge) SMSOTPChallenge {
	return SMSOTPChallenge{
		ChallengeID: row.ChallengeID,
		PhoneHash:   row.PhoneHash,
		CodeDigest:  row.CodeDigest,
		ExpiresAt:   row.ExpiresAt.UTC(),
		Attempts:    row.Attempts,
		MessageID:   row.MessageID,
		UsedAt:      timeFromPointer(row.UsedAt),
		Status:      RecordStatus(row.Status),
	}
}

func timePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}

func timeFromPointer(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return value.UTC()
}
