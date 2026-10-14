package utility

import (
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func withSecret(t *testing.T, key string) {
	t.Helper()
	old := secretKey
	secretKey = []byte(key)
	t.Cleanup(func() { secretKey = old })
}

func TestCreateAndVerifyToken(t *testing.T) {
	withSecret(t, "test-secret")
	id := uuid.New()

	token, err := CreateToken(id, "seller@example.com")
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	claims, err := VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if claims.AdminId != id.String() {
		t.Errorf("AdminId = %q, want %q", claims.AdminId, id)
	}
	if claims.Email != "seller@example.com" {
		t.Errorf("Email = %q", claims.Email)
	}
}

func TestVerifyToken_WrongSecretRejected(t *testing.T) {
	withSecret(t, "secret-a")
	token, _ := CreateToken(uuid.New(), "a@b.com")

	secretKey = []byte("secret-b")
	if _, err := VerifyToken(token); err == nil {
		t.Fatal("token signed with a different secret was accepted")
	}
}

func TestVerifyToken_ExpiredRejected(t *testing.T) {
	withSecret(t, "test-secret")
	claims := jwt.MapClaims{
		"admin_id": uuid.NewString(),
		"email":    "a@b.com",
		"exp":      time.Now().Add(-time.Minute).Unix(),
	}
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secretKey)

	if _, err := VerifyToken(token); err == nil {
		t.Fatal("expired token was accepted")
	}
}

func TestVerifyToken_NoneAlgRejected(t *testing.T) {
	withSecret(t, "test-secret")
	claims := jwt.MapClaims{"admin_id": uuid.NewString(), "exp": time.Now().Add(time.Hour).Unix()}
	token, _ := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)

	if _, err := VerifyToken(token); err == nil {
		t.Fatal("unsigned (alg=none) token was accepted")
	}
}

func TestVerifyToken_Garbage(t *testing.T) {
	withSecret(t, "test-secret")
	if _, err := VerifyToken("not.a.jwt"); err == nil {
		t.Fatal("garbage token was accepted")
	}
}

func TestHashAndComparePassword(t *testing.T) {
	hash, err := HashPassword("correct-horse")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "correct-horse" {
		t.Fatal("password stored in plain text")
	}
	if err := ComparePassword(hash, "correct-horse"); err != nil {
		t.Errorf("correct password rejected: %v", err)
	}
	if err := ComparePassword(hash, "wrong"); err == nil {
		t.Error("wrong password accepted")
	}
}

func TestGenerateAPIKey(t *testing.T) {
	a, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	b, _ := GenerateAPIKey()
	if len(a) != 64 {
		t.Errorf("key length = %d, want 64 hex chars", len(a))
	}
	if a == b {
		t.Error("two generated keys are identical")
	}
}

func TestHashAPIKey(t *testing.T) {
	key := "abc123"
	if HashAPIKey(key) != HashAPIKey(key) {
		t.Error("hash is not deterministic")
	}
	if HashAPIKey(key) == key {
		t.Error("hash equals the raw key")
	}
	if HashAPIKey(key) == HashAPIKey("abc124") {
		t.Error("different keys produced the same hash")
	}
}

func TestStringToUUID(t *testing.T) {
	id := uuid.New()
	got, err := StringToUUID(id.String())
	if err != nil || got != id {
		t.Errorf("StringToUUID(%q) = %v, %v", id, got, err)
	}
	if _, err := StringToUUID("not-a-uuid"); err == nil {
		t.Error("invalid uuid accepted")
	}
}

func TestValidationErrors(t *testing.T) {
	type req struct {
		Email    string `validate:"required,email"`
		Password string `validate:"required,min=6"`
		Name     string `validate:"required"`
	}
	err := Validate.Struct(req{Email: "nope", Password: "123"})
	if _, ok := err.(validator.ValidationErrors); !ok {
		t.Fatalf("expected validator.ValidationErrors, got %T", err)
	}

	msg := ValidationErrors(err).Error()
	for _, want := range []string{"Invalid email format", "Name is required", "at least 6"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q missing %q", msg, want)
		}
	}
}
