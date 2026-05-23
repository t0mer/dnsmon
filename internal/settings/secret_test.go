package settings

import "testing"

func TestHashVerifySecret(t *testing.T) {
	hash, err := HashSecret("s3cret-password")
	if err != nil {
		t.Fatalf("HashSecret: %v", err)
	}
	if hash == "s3cret-password" {
		t.Fatal("hash must not equal the plaintext")
	}

	if !VerifySecret("s3cret-password", hash) {
		t.Error("VerifySecret returned false for the correct secret")
	}
	if VerifySecret("wrong-password", hash) {
		t.Error("VerifySecret returned true for a wrong secret")
	}
}

func TestVerifySecretRejectsGarbage(t *testing.T) {
	for _, bad := range []string{"", "not-a-hash", "$argon2id$bad", "$bcrypt$v=1$x$y$z"} {
		if VerifySecret("x", bad) {
			t.Errorf("VerifySecret(%q) = true, want false", bad)
		}
	}
}

func TestGenerateTokenUnique(t *testing.T) {
	a, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	b, _ := GenerateToken()
	if a == "" || a == b {
		t.Errorf("tokens should be non-empty and unique: %q vs %q", a, b)
	}
}
