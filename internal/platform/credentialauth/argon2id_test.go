package credentialauth

import "testing"

func TestArgon2idVerifierAcceptsPHCAndReportsRehash(t *testing.T) {
	storedParams := Argon2idParams{MemoryKiB: 64, Iterations: 1, Parallelism: 1, SaltLength: 8, KeyLength: 16}
	encoded, err := HashPasswordArgon2id("correct horse battery staple", storedParams)
	if err != nil {
		t.Fatalf("HashPasswordArgon2id() error = %v", err)
	}
	verifier := NewArgon2idVerifier(Argon2idParams{MemoryKiB: 128, Iterations: 1, Parallelism: 1, KeyLength: 16})
	ok, needsRehash, err := verifier.Verify(encoded, "correct horse battery staple")
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if !ok || !needsRehash {
		t.Fatalf("Verify() = %t, %t; want ok and needs rehash", ok, needsRehash)
	}
}

func TestArgon2idVerifierRejectsWrongPasswordAndMalformedHash(t *testing.T) {
	params := Argon2idParams{MemoryKiB: 64, Iterations: 1, Parallelism: 1, SaltLength: 8, KeyLength: 16}
	encoded, err := HashPasswordArgon2id("correct", params)
	if err != nil {
		t.Fatalf("HashPasswordArgon2id() error = %v", err)
	}
	verifier := NewArgon2idVerifier(params)
	ok, needsRehash, err := verifier.Verify(encoded, "wrong")
	if err != nil {
		t.Fatalf("Verify(wrong) error = %v", err)
	}
	if ok || needsRehash {
		t.Fatalf("Verify(wrong) = %t, %t; want false, false", ok, needsRehash)
	}
	if _, _, err := verifier.Verify("not-a-phc", "correct"); err != ErrInvalidSecret {
		t.Fatalf("Verify(malformed) error = %v, want ErrInvalidSecret", err)
	}
}
