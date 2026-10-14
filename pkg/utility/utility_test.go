package utility

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func withSecret(t *testing.T, key string) {
	t.Helper()
	t.Setenv("SECRET_KEY", key)
}

func sign(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	key, err := secretKey()
	if err != nil {
		t.Fatal(err)
	}
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
	return token
}

func TestCreateAndVerifyToken(t *testing.T) {
	withSecret(t, "test-secret")
	id := uuid.New()

	token, err := CreateToken(id, "seller@example.com", RoleAdmin)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	claims, err := VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if claims.UserId != id.String() {
		t.Errorf("UserId = %q, want %q", claims.UserId, id)
	}
	if claims.Role != RoleAdmin {
		t.Errorf("Role = %q, want %q", claims.Role, RoleAdmin)
	}
	if claims.Email != "seller@example.com" {
		t.Errorf("Email = %q", claims.Email)
	}
}

func TestVerifyToken_WrongSecretRejected(t *testing.T) {
	withSecret(t, "secret-a")
	token, _ := CreateToken(uuid.New(), "a@b.com", RoleAdmin)

	withSecret(t, "secret-b")
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
	if _, err := VerifyToken(sign(t, claims)); err == nil {
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

func TestToken_MissingSecret(t *testing.T) {
	withSecret(t, "")
	if _, err := CreateToken(uuid.New(), "a@b.com", RoleAdmin); err != ErrMissingSecret {
		t.Errorf("CreateToken err = %v, want ErrMissingSecret", err)
	}

	// A token signed with an empty key must never verify.
	forged, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": uuid.NewString(), "role": RoleAdmin, "exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte(""))
	if _, err := VerifyToken(forged); err == nil {
		t.Error("token signed with empty key was accepted")
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

func validationMap(t *testing.T, v any) map[string]string {
	t.Helper()
	err := Validate.Struct(v)
	if err == nil {
		t.Fatalf("expected validation error for %+v", v)
	}
	got := map[string]string{}
	if jsonErr := json.Unmarshal([]byte(ValidationErrors(err).Error()), &got); jsonErr != nil {
		t.Fatalf("ValidationErrors did not return a JSON object: %v", jsonErr)
	}
	return got
}

func TestValidationErrors_Messages(t *testing.T) {
	type req struct {
		Email       string   `json:"email" validate:"required,email"`
		Password    string   `json:"password" validate:"required,min=6"`
		Name        string   `json:"name" validate:"required"`
		Description string   `json:"store_description" validate:"min=10"`
		Currency    string   `json:"currency" validate:"len=3"`
		Quantity    int      `json:"quantity" validate:"gte=1"`
		Tags        []string `json:"tags" validate:"max=2"`
		Code        string   `json:"code" validate:"uppercase"`
		NoJSONTag   string   `validate:"required"`
	}
	got := validationMap(t, req{Email: "nope", Password: "123", Description: "short", Currency: "NGNX", Tags: []string{"a", "b", "c"}, Code: "abc"})

	want := map[string]string{
		"email":             "email must be a valid email address",
		"password":          "password must be at least 6 characters",
		"name":              "name is required",
		"store_description": "store_description must be at least 10 characters",
		"currency":          "currency must be exactly 3 characters",
		"quantity":          "quantity must be at least 1",
		"tags":              "tags must be at most 2 items",
		"code":              "code is invalid (uppercase)",
		"NoJSONTag":         "NoJSONTag is required",
	}
	for field, msg := range want {
		if got[field] != msg {
			t.Errorf("%s: got %q, want %q", field, got[field], msg)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d errors, want %d: %v", len(got), len(want), got)
	}
}

// A non-validation error (e.g. passing a non-struct) used to panic.
func TestValidationErrors_OtherErrorReturnedUnchanged(t *testing.T) {
	err := Validate.Struct(42)
	if err == nil {
		t.Fatal("expected an error validating a non-struct")
	}
	if got := ValidationErrors(err); got != err {
		t.Errorf("got %v, want the original error", got)
	}

	plain := errors.New("boom")
	if got := ValidationErrors(plain); got != plain {
		t.Errorf("got %v, want the original error", got)
	}
}
