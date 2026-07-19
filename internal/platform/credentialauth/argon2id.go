package credentialauth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	PasswordAlgorithmArgon2id = "argon2id"
	argon2idVersion           = 19
)

type Argon2idParams struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

type Argon2idVerifier struct {
	Params Argon2idParams
}

func DefaultArgon2idParams() Argon2idParams {
	return Argon2idParams{MemoryKiB: 64 * 1024, Iterations: 3, Parallelism: 1, SaltLength: 16, KeyLength: 32}
}

func NewArgon2idVerifier(params Argon2idParams) Argon2idVerifier {
	return Argon2idVerifier{Params: normalizeArgon2idParams(params)}
}

func HashPasswordArgon2id(password string, params Argon2idParams) (string, error) {
	if strings.TrimSpace(password) == "" {
		return "", ErrInvalidInput
	}
	params = normalizeArgon2idParams(params)
	salt := make([]byte, params.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, params.Iterations, params.MemoryKiB, params.Parallelism, params.KeyLength)
	return encodeArgon2idPHC(params, salt, key), nil
}

func (v Argon2idVerifier) Verify(encodedHash string, plainSecret string) (bool, bool, error) {
	if strings.TrimSpace(plainSecret) == "" {
		return false, false, ErrInvalidInput
	}
	params, salt, key, err := parseArgon2idPHC(encodedHash)
	if err != nil {
		return false, false, err
	}
	actual := argon2.IDKey([]byte(plainSecret), salt, params.Iterations, params.MemoryKiB, params.Parallelism, uint32(len(key)))
	if subtle.ConstantTimeCompare(actual, key) != 1 {
		return false, false, nil
	}
	target := normalizeArgon2idParams(v.Params)
	needsRehash := params.MemoryKiB != target.MemoryKiB || params.Iterations != target.Iterations || params.Parallelism != target.Parallelism || uint32(len(key)) != target.KeyLength
	return true, needsRehash, nil
}

func (v Argon2idVerifier) VerifyPassword(_ context.Context, credential PasswordCredential, plainSecret string) (PasswordVerification, error) {
	if strings.TrimSpace(credential.Algorithm) != PasswordAlgorithmArgon2id {
		return PasswordVerification{}, ErrInvalidSecret
	}
	valid, needsRehash, err := v.Verify(credential.PasswordHash, plainSecret)
	if err != nil || !valid {
		return PasswordVerification{Valid: false}, err
	}
	if !needsRehash {
		return PasswordVerification{Valid: true}, nil
	}
	replacementHash, err := HashPasswordArgon2id(plainSecret, v.Params)
	if err != nil {
		return PasswordVerification{}, err
	}
	paramsVersion := strings.TrimSpace(credential.ParamsVersion)
	if paramsVersion == "" {
		paramsVersion = "argon2id-default"
	}
	return PasswordVerification{
		Valid:           true,
		NeedsRehash:     true,
		ReplacementHash: replacementHash,
		Algorithm:       PasswordAlgorithmArgon2id,
		ParamsVersion:   paramsVersion,
	}, nil
}

func normalizeArgon2idParams(params Argon2idParams) Argon2idParams {
	defaults := DefaultArgon2idParams()
	if params.MemoryKiB == 0 {
		params.MemoryKiB = defaults.MemoryKiB
	}
	if params.Iterations == 0 {
		params.Iterations = defaults.Iterations
	}
	if params.Parallelism == 0 {
		params.Parallelism = defaults.Parallelism
	}
	if params.SaltLength == 0 {
		params.SaltLength = defaults.SaltLength
	}
	if params.KeyLength == 0 {
		params.KeyLength = defaults.KeyLength
	}
	return params
}

func encodeArgon2idPHC(params Argon2idParams, salt []byte, key []byte) string {
	encoding := base64.RawStdEncoding
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2idVersion, params.MemoryKiB, params.Iterations, params.Parallelism,
		encoding.EncodeToString(salt), encoding.EncodeToString(key),
	)
}

func parseArgon2idPHC(encodedHash string) (Argon2idParams, []byte, []byte, error) {
	parts := strings.Split(strings.TrimSpace(encodedHash), "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != PasswordAlgorithmArgon2id {
		return Argon2idParams{}, nil, nil, ErrInvalidSecret
	}
	if parts[2] != fmt.Sprintf("v=%d", argon2idVersion) {
		return Argon2idParams{}, nil, nil, ErrInvalidSecret
	}
	params, err := parseArgon2idParamSet(parts[3])
	if err != nil {
		return Argon2idParams{}, nil, nil, err
	}
	encoding := base64.RawStdEncoding
	salt, err := encoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return Argon2idParams{}, nil, nil, ErrInvalidSecret
	}
	key, err := encoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return Argon2idParams{}, nil, nil, ErrInvalidSecret
	}
	params.SaltLength = uint32(len(salt))
	params.KeyLength = uint32(len(key))
	return params, salt, key, nil
}

func parseArgon2idParamSet(value string) (Argon2idParams, error) {
	fields := strings.Split(value, ",")
	if len(fields) != 3 {
		return Argon2idParams{}, ErrInvalidSecret
	}
	values := map[string]string{}
	for _, field := range fields {
		key, raw, ok := strings.Cut(field, "=")
		if !ok || key == "" || raw == "" {
			return Argon2idParams{}, ErrInvalidSecret
		}
		values[key] = raw
	}
	memory, err := parsePositiveUint32(values["m"])
	if err != nil {
		return Argon2idParams{}, ErrInvalidSecret
	}
	iterations, err := parsePositiveUint32(values["t"])
	if err != nil {
		return Argon2idParams{}, ErrInvalidSecret
	}
	parallelism64, err := strconv.ParseUint(values["p"], 10, 8)
	if err != nil || parallelism64 == 0 {
		return Argon2idParams{}, ErrInvalidSecret
	}
	return Argon2idParams{MemoryKiB: memory, Iterations: iterations, Parallelism: uint8(parallelism64)}, nil
}

func parsePositiveUint32(value string) (uint32, error) {
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil || parsed == 0 {
		return 0, ErrInvalidSecret
	}
	return uint32(parsed), nil
}
