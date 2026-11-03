package customer

import (
	"net/http"
	"testing"

	"huddle/pkg/model"
	"huddle/pkg/testutil"
)

func TestSignup_DuplicateEmailDifferentCase(t *testing.T) {
	t.Setenv("SECRET_KEY", "test-secret")
	db := testutil.NewDB(t)
	_, rdb := testutil.NewRedis(t)
	admin := testutil.SeedAdmin(t, db)
	store := testutil.SeedStore(t, db, admin.Id)
	other := testutil.SeedStore(t, db, admin.Id)

	req := model.CustomerSignupReq{StoreId: store.StoreId, Name: "Ada", Email: "ada@example.com", Password: "secret123"}
	if _, _, err := Signup(db, rdb, req); err != nil {
		t.Fatalf("first signup: %s %v", err.Message, err.Err)
	}

	req.Email = "Ada@Example.com"
	if _, _, err := Signup(db, rdb, req); err == nil || err.StatusCode != http.StatusConflict {
		t.Errorf("same store, different case: err = %+v, want 409", err)
	}

	// The same email may shop at another store.
	req.StoreId = other.StoreId
	if _, _, err := Signup(db, rdb, req); err != nil {
		t.Errorf("other store, same email: %s", err.Message)
	}
}

func TestSignin_EmailIgnoresCase(t *testing.T) {
	t.Setenv("SECRET_KEY", "test-secret")
	db := testutil.NewDB(t)
	_, rdb := testutil.NewRedis(t)
	admin := testutil.SeedAdmin(t, db)
	store := testutil.SeedStore(t, db, admin.Id)

	signup := model.CustomerSignupReq{StoreId: store.StoreId, Name: "Ada", Email: "ada@example.com", Password: "secret123"}
	if _, _, err := Signup(db, rdb, signup); err != nil {
		t.Fatalf("signup: %s %v", err.Message, err.Err)
	}

	req := model.CustomerSigninReq{StoreId: store.StoreId, Email: "Ada@Example.COM", Password: "secret123"}
	if _, _, err := Signin(db, rdb, req); err != nil {
		t.Errorf("signin with different case: %s", err.Message)
	}
	req.Password = "wrong-password"
	if _, _, err := Signin(db, rdb, req); err == nil || err.StatusCode != http.StatusUnauthorized {
		t.Errorf("wrong password: err = %+v, want 401", err)
	}
}
