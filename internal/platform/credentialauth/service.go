package credentialauth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type PasswordVerifier interface {
	VerifyPassword(context.Context, PasswordCredential, string) (PasswordVerification, error)
}

type PasswordVerification struct {
	Valid           bool
	NeedsRehash     bool
	ReplacementHash string
	Algorithm       string
	ParamsVersion   string
}

type Options struct {
	Repository                 Repository
	IdentifierHasher           IdentifierHasher
	PasswordVerifier           PasswordVerifier
	ChallengeProofHasher       ChallengeProofHasher
	ChallengeMaterialGenerator func(ChallengeKind) (ChallengeMaterial, error)
	ChallengeIDGenerator       func() (string, error)
	Now                        func() time.Time
	MaxPasswordAttempts        int
	PasswordLock               time.Duration
	ChallengeTTL               time.Duration
}

type Service struct {
	repository                 Repository
	identifierHasher           IdentifierHasher
	passwordVerifier           PasswordVerifier
	challengeProofHasher       ChallengeProofHasher
	challengeMaterialGenerator func(ChallengeKind) (ChallengeMaterial, error)
	challengeIDGenerator       func() (string, error)
	now                        func() time.Time
	maxPasswordAttempts        int
	passwordLock               time.Duration
	challengeTTL               time.Duration
}

type RegisterIdentifierInput struct {
	Principal  PrincipalRef
	Identifier Identifier
	VerifiedAt time.Time
	Status     RecordStatus
}

type PasswordLoginInput struct {
	Identifier Identifier
	Secret     string
}

type PasswordLoginResult struct {
	Principal     PrincipalRef
	MustChange    bool
	Rehashed      bool
	LockedUntil   time.Time
	ParamsVersion string
}

type CredentialChallengeProof struct {
	ChallengeID           string
	Kind                  ChallengeKind
	Purpose               ChallengePurpose
	Proof                 string
	AnswerDigest          string
	ClientFingerprintHash string
	MaxAttempts           int
}

type ChallengeMaterial struct {
	Prompt     string
	Parameters map[string]string
	Proof      string
}

type CreateCredentialChallengeInput struct {
	Kind                  ChallengeKind
	Purpose               ChallengePurpose
	ClientFingerprintHash string
	TTL                   time.Duration
}

type CreatedCredentialChallenge struct {
	ChallengeID string
	Kind        ChallengeKind
	Purpose     ChallengePurpose
	Prompt      string
	Parameters  map[string]string
	ExpiresAt   time.Time
}

type SMSOTPProof struct {
	ChallengeID string
	PhoneHash   string
	CodeDigest  string
	MaxAttempts int
}

func NewService(options Options) (*Service, error) {
	if options.Repository == nil {
		return nil, fmt.Errorf("%w: repository is required", ErrInvalidInput)
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	maxPasswordAttempts := options.MaxPasswordAttempts
	if maxPasswordAttempts == 0 {
		maxPasswordAttempts = DefaultMaxPasswordAttempts
	}
	if maxPasswordAttempts < 0 {
		return nil, fmt.Errorf("%w: max password attempts must not be negative", ErrInvalidInput)
	}
	passwordLock := options.PasswordLock
	if passwordLock == 0 {
		passwordLock = DefaultPasswordLockDuration
	}
	if passwordLock < 0 {
		return nil, fmt.Errorf("%w: password lock duration must not be negative", ErrInvalidInput)
	}
	challengeTTL := options.ChallengeTTL
	if challengeTTL == 0 {
		challengeTTL = DefaultCredentialChallengeTTL
	}
	if challengeTTL < 0 {
		return nil, fmt.Errorf("%w: challenge ttl must not be negative", ErrInvalidInput)
	}
	challengeMaterialGenerator := options.ChallengeMaterialGenerator
	if challengeMaterialGenerator == nil {
		challengeMaterialGenerator = defaultChallengeMaterial
	}
	challengeIDGenerator := options.ChallengeIDGenerator
	if challengeIDGenerator == nil {
		challengeIDGenerator = newCredentialChallengeID
	}
	return &Service{
		repository:                 options.Repository,
		identifierHasher:           options.IdentifierHasher,
		passwordVerifier:           options.PasswordVerifier,
		challengeProofHasher:       options.ChallengeProofHasher,
		challengeMaterialGenerator: challengeMaterialGenerator,
		challengeIDGenerator:       challengeIDGenerator,
		now:                        now,
		maxPasswordAttempts:        maxPasswordAttempts,
		passwordLock:               passwordLock,
		challengeTTL:               challengeTTL,
	}, nil
}

func (s *Service) RegisterIdentifier(ctx context.Context, input RegisterIdentifierInput) (IdentifierRecord, error) {
	if err := checkContext(ctx); err != nil {
		return IdentifierRecord{}, err
	}
	if s == nil || s.identifierHasher == nil {
		return IdentifierRecord{}, fmt.Errorf("%w: identifier hasher is required", ErrInvalidInput)
	}
	normalized, err := NormalizeIdentifier(input.Identifier)
	if err != nil {
		return IdentifierRecord{}, err
	}
	identifierHash, err := s.identifierHasher.HashIdentifier(normalized.Type, normalized.Value)
	if err != nil {
		return IdentifierRecord{}, err
	}
	verifiedAt := input.VerifiedAt
	if verifiedAt.IsZero() {
		verifiedAt = s.now().UTC()
	}
	principal, err := input.Principal.normalized()
	if err != nil {
		return IdentifierRecord{}, err
	}
	record := IdentifierRecord{
		Principal:        principal,
		IdentifierType:   normalized.Type,
		IdentifierHash:   identifierHash,
		MaskedIdentifier: normalized.MaskedIdentifier,
		VerifiedAt:       verifiedAt.UTC(),
		Status:           normalizeStatus(input.Status),
	}
	if err := s.repository.UpsertIdentifier(ctx, record); err != nil {
		return IdentifierRecord{}, err
	}
	return record, nil
}

func (s *Service) ResolveIdentifier(ctx context.Context, identifier Identifier) (IdentifierRecord, bool, error) {
	if err := checkContext(ctx); err != nil {
		return IdentifierRecord{}, false, err
	}
	if s == nil || s.identifierHasher == nil {
		return IdentifierRecord{}, false, fmt.Errorf("%w: identifier hasher is required", ErrInvalidInput)
	}
	normalized, err := NormalizeIdentifier(identifier)
	if err != nil {
		return IdentifierRecord{}, false, err
	}
	identifierHash, err := s.identifierHasher.HashIdentifier(normalized.Type, normalized.Value)
	if err != nil {
		return IdentifierRecord{}, false, err
	}
	return s.repository.LookupIdentifier(ctx, normalized.Type, identifierHash)
}

func (s *Service) PutPasswordCredential(ctx context.Context, credential PasswordCredential) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("%w: service is required", ErrInvalidInput)
	}
	if credential.PasswordUpdatedAt.IsZero() {
		credential.PasswordUpdatedAt = s.now().UTC()
	}
	return s.repository.UpsertPasswordCredential(ctx, credential)
}

func (s *Service) VerifyPassword(ctx context.Context, input PasswordLoginInput) (PasswordLoginResult, error) {
	if err := checkContext(ctx); err != nil {
		return PasswordLoginResult{}, err
	}
	if s == nil || s.passwordVerifier == nil {
		return PasswordLoginResult{}, fmt.Errorf("%w: password verifier is required", ErrInvalidInput)
	}
	if strings.TrimSpace(input.Secret) == "" {
		return PasswordLoginResult{}, ErrCredentialRejected
	}
	identifier, ok, err := s.ResolveIdentifier(ctx, input.Identifier)
	if err != nil {
		return PasswordLoginResult{}, err
	}
	if !ok || normalizeStatus(identifier.Status) != StatusEnabled {
		return PasswordLoginResult{}, ErrCredentialRejected
	}
	credential, ok, err := s.repository.PasswordCredential(ctx, identifier.Principal)
	if err != nil {
		return PasswordLoginResult{}, err
	}
	if !ok || normalizeStatus(credential.Status) != StatusEnabled || !samePrincipal(identifier.Principal, credential.Principal) {
		return PasswordLoginResult{}, ErrCredentialRejected
	}
	now := s.now().UTC()
	if !credential.LockedUntil.IsZero() && now.Before(credential.LockedUntil) {
		return PasswordLoginResult{Principal: credential.Principal, LockedUntil: credential.LockedUntil}, ErrCredentialLocked
	}
	verification, err := s.passwordVerifier.VerifyPassword(ctx, credential, input.Secret)
	if err != nil {
		return PasswordLoginResult{}, err
	}
	if !verification.Valid {
		credential.FailedAttempts++
		if s.maxPasswordAttempts > 0 && credential.FailedAttempts >= s.maxPasswordAttempts {
			credential.LockedUntil = now.Add(s.passwordLock)
		}
		if err := s.repository.UpsertPasswordCredential(ctx, credential); err != nil {
			return PasswordLoginResult{}, err
		}
		return PasswordLoginResult{Principal: credential.Principal, LockedUntil: credential.LockedUntil}, ErrCredentialRejected
	}
	rehashed := false
	credential.FailedAttempts = 0
	credential.LockedUntil = time.Time{}
	if verification.NeedsRehash {
		if strings.TrimSpace(verification.ReplacementHash) == "" || strings.TrimSpace(verification.Algorithm) == "" || strings.TrimSpace(verification.ParamsVersion) == "" {
			return PasswordLoginResult{}, fmt.Errorf("%w: password rehash result is invalid", ErrRepositoryInvariant)
		}
		credential.PasswordHash = strings.TrimSpace(verification.ReplacementHash)
		credential.Algorithm = strings.TrimSpace(verification.Algorithm)
		credential.ParamsVersion = strings.TrimSpace(verification.ParamsVersion)
		credential.PasswordUpdatedAt = now
		rehashed = true
	}
	if err := s.repository.UpsertPasswordCredential(ctx, credential); err != nil {
		return PasswordLoginResult{}, err
	}
	return PasswordLoginResult{
		Principal:     credential.Principal,
		MustChange:    credential.MustChange,
		Rehashed:      rehashed,
		ParamsVersion: credential.ParamsVersion,
	}, nil
}

func (s *Service) PutCredentialChallenge(ctx context.Context, challenge CredentialChallenge) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("%w: service is required", ErrInvalidInput)
	}
	return s.repository.UpsertCredentialChallenge(ctx, challenge)
}

func (s *Service) CreateCredentialChallenge(ctx context.Context, input CreateCredentialChallengeInput) (CreatedCredentialChallenge, error) {
	if err := checkContext(ctx); err != nil {
		return CreatedCredentialChallenge{}, err
	}
	if s == nil || s.challengeProofHasher == nil {
		return CreatedCredentialChallenge{}, fmt.Errorf("%w: challenge proof hasher is required", ErrInvalidInput)
	}
	kind := ChallengeKind(strings.TrimSpace(string(input.Kind)))
	if kind == "" {
		kind = ChallengeKindCaptcha
	}
	if !validChallengeKind(kind) {
		return CreatedCredentialChallenge{}, fmt.Errorf("%w: challenge kind is invalid", ErrInvalidInput)
	}
	purpose := ChallengePurpose(strings.TrimSpace(string(input.Purpose)))
	if purpose == "" {
		purpose = ChallengePurposeLogin
	}
	if !validChallengePurpose(purpose) {
		return CreatedCredentialChallenge{}, fmt.Errorf("%w: challenge purpose is invalid", ErrInvalidInput)
	}
	ttl := input.TTL
	if ttl == 0 {
		ttl = s.challengeTTL
	}
	if ttl <= 0 {
		return CreatedCredentialChallenge{}, fmt.Errorf("%w: challenge ttl is invalid", ErrInvalidInput)
	}
	challengeID, err := s.challengeIDGenerator()
	if err != nil {
		return CreatedCredentialChallenge{}, err
	}
	challengeID = strings.TrimSpace(challengeID)
	if challengeID == "" {
		return CreatedCredentialChallenge{}, fmt.Errorf("%w: challenge id generator returned an empty id", ErrRepositoryInvariant)
	}
	material, err := s.challengeMaterialGenerator(kind)
	if err != nil {
		return CreatedCredentialChallenge{}, err
	}
	material.Proof = strings.TrimSpace(material.Proof)
	material.Prompt = strings.TrimSpace(material.Prompt)
	if material.Proof == "" || material.Prompt == "" {
		return CreatedCredentialChallenge{}, fmt.Errorf("%w: challenge material is invalid", ErrRepositoryInvariant)
	}
	answerDigest, err := s.challengeProofHasher.HashChallengeProof(kind, purpose, challengeID, material.Proof)
	if err != nil {
		return CreatedCredentialChallenge{}, err
	}
	now := s.now().UTC()
	if err := s.repository.UpsertCredentialChallenge(ctx, CredentialChallenge{
		ChallengeID:           challengeID,
		Kind:                  kind,
		Purpose:               purpose,
		AnswerDigest:          answerDigest,
		ExpiresAt:             now.Add(ttl),
		ClientFingerprintHash: strings.TrimSpace(input.ClientFingerprintHash),
		Status:                StatusEnabled,
	}); err != nil {
		return CreatedCredentialChallenge{}, err
	}
	return CreatedCredentialChallenge{
		ChallengeID: challengeID,
		Kind:        kind,
		Purpose:     purpose,
		Prompt:      material.Prompt,
		Parameters:  cloneStringMap(material.Parameters),
		ExpiresAt:   now.Add(ttl),
	}, nil
}

func (s *Service) VerifyCredentialChallenge(ctx context.Context, input CredentialChallengeProof) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("%w: service is required", ErrInvalidInput)
	}
	challenge, ok, err := s.repository.CredentialChallenge(ctx, input.ChallengeID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrChallengeRejected
	}
	now := s.now().UTC()
	if err := validateActiveChallenge(challenge.Status, challenge.UsedAt, challenge.ExpiresAt, now); err != nil {
		return err
	}
	if input.Kind != "" && input.Kind != challenge.Kind {
		return s.rejectCredentialChallenge(ctx, challenge, input.MaxAttempts)
	}
	if input.Purpose != "" && input.Purpose != challenge.Purpose {
		return s.rejectCredentialChallenge(ctx, challenge, input.MaxAttempts)
	}
	if challenge.ClientFingerprintHash != "" && input.ClientFingerprintHash != challenge.ClientFingerprintHash {
		return s.rejectCredentialChallenge(ctx, challenge, input.MaxAttempts)
	}
	answerDigest := strings.TrimSpace(input.AnswerDigest)
	if answerDigest == "" && strings.TrimSpace(input.Proof) != "" {
		if s.challengeProofHasher == nil {
			return fmt.Errorf("%w: challenge proof hasher is required", ErrInvalidInput)
		}
		digest, err := s.challengeProofHasher.HashChallengeProof(challenge.Kind, challenge.Purpose, challenge.ChallengeID, input.Proof)
		if err != nil {
			return err
		}
		answerDigest = digest
	}
	if answerDigest == "" || subtle.ConstantTimeCompare([]byte(answerDigest), []byte(challenge.AnswerDigest)) != 1 {
		return s.rejectCredentialChallenge(ctx, challenge, input.MaxAttempts)
	}
	challenge.UsedAt = now
	return s.repository.UpsertCredentialChallenge(ctx, challenge)
}

func (s *Service) PutSMSOTPChallenge(ctx context.Context, challenge SMSOTPChallenge) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("%w: service is required", ErrInvalidInput)
	}
	return s.repository.UpsertSMSOTPChallenge(ctx, challenge)
}

func (s *Service) VerifySMSOTP(ctx context.Context, input SMSOTPProof) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("%w: service is required", ErrInvalidInput)
	}
	challenge, ok, err := s.repository.SMSOTPChallenge(ctx, input.ChallengeID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrChallengeRejected
	}
	now := s.now().UTC()
	if err := validateActiveChallenge(challenge.Status, challenge.UsedAt, challenge.ExpiresAt, now); err != nil {
		return err
	}
	if strings.TrimSpace(input.PhoneHash) != challenge.PhoneHash || strings.TrimSpace(input.CodeDigest) == "" || subtle.ConstantTimeCompare([]byte(strings.TrimSpace(input.CodeDigest)), []byte(challenge.CodeDigest)) != 1 {
		return s.rejectSMSOTP(ctx, challenge, input.MaxAttempts)
	}
	challenge.UsedAt = now
	return s.repository.UpsertSMSOTPChallenge(ctx, challenge)
}

func (s *Service) rejectCredentialChallenge(ctx context.Context, challenge CredentialChallenge, maxAttempts int) error {
	if maxAttempts == 0 {
		maxAttempts = DefaultMaxChallengeAttempts
	}
	if maxAttempts < 0 {
		return fmt.Errorf("%w: max challenge attempts must not be negative", ErrInvalidInput)
	}
	if maxAttempts > 0 && challenge.Attempts >= maxAttempts {
		return ErrChallengeRejected
	}
	challenge.Attempts++
	if err := s.repository.UpsertCredentialChallenge(ctx, challenge); err != nil {
		return err
	}
	return ErrChallengeRejected
}

func (s *Service) rejectSMSOTP(ctx context.Context, challenge SMSOTPChallenge, maxAttempts int) error {
	if maxAttempts == 0 {
		maxAttempts = DefaultMaxSMSOTPAttempts
	}
	if maxAttempts < 0 {
		return fmt.Errorf("%w: max sms otp attempts must not be negative", ErrInvalidInput)
	}
	if maxAttempts > 0 && challenge.Attempts >= maxAttempts {
		return ErrChallengeRejected
	}
	challenge.Attempts++
	if err := s.repository.UpsertSMSOTPChallenge(ctx, challenge); err != nil {
		return err
	}
	return ErrChallengeRejected
}

func validateActiveChallenge(status RecordStatus, usedAt time.Time, expiresAt time.Time, now time.Time) error {
	if normalizeStatus(status) != StatusEnabled {
		return ErrChallengeRejected
	}
	if !usedAt.IsZero() {
		return ErrChallengeConsumed
	}
	if !now.Before(expiresAt) {
		return ErrChallengeExpired
	}
	return nil
}

func samePrincipal(first PrincipalRef, second PrincipalRef) bool {
	first, firstErr := first.normalized()
	second, secondErr := second.normalized()
	return firstErr == nil && secondErr == nil && first.Type == second.Type && first.ID == second.ID
}

func defaultChallengeMaterial(kind ChallengeKind) (ChallengeMaterial, error) {
	switch kind {
	case ChallengeKindCaptcha:
		return defaultTextCaptchaMaterial()
	case ChallengeKindSlider:
		generator, err := NewGoCaptchaSlideMaterialGenerator(GoCaptchaSlideOptions{})
		if err != nil {
			return ChallengeMaterial{}, err
		}
		return generator(kind)
	default:
		return ChallengeMaterial{}, fmt.Errorf("%w: challenge kind is invalid", ErrInvalidInput)
	}
}

func randomChallengeToken(length int) (string, error) {
	const alphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	if length <= 0 {
		return "", fmt.Errorf("%w: challenge token length is invalid", ErrInvalidInput)
	}
	raw := make([]byte, length)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	builder := strings.Builder{}
	builder.Grow(length)
	for _, value := range raw {
		builder.WriteByte(alphabet[int(value)%len(alphabet)])
	}
	return builder.String(), nil
}

func randomSliderOffset() (string, error) {
	var raw [1]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", 24+int(raw[0])%153), nil
}

func newCredentialChallengeID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "challenge-" + hex.EncodeToString(raw[:]), nil
}

func cloneStringMap(items map[string]string) map[string]string {
	if len(items) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(items))
	for key, value := range items {
		cloned[key] = value
	}
	return cloned
}
