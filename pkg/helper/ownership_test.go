package helper

import (
	"net/http"
	"testing"

	"huddle/pkg/testutil"

	"github.com/google/uuid"
)

func TestVerifyProductOwnership(t *testing.T) {
	db := testutil.NewDB(t)
	_, rdb := testutil.NewRedis(t)

	owner := testutil.SeedAdmin(t, db)
	other := testutil.SeedAdmin(t, db)
	store := testutil.SeedStore(t, db, owner.Id)
	product := testutil.SeedProduct(t, db, store.ID, "Ankara bag", 15000, 5)

	id, err := VerifyProductOwnership(db, rdb, product.PublicId, owner.Id.String())
	if err != nil {
		t.Fatalf("owner rejected: %s %v", err.Message, err.Err)
	}
	if *id != product.Id {
		t.Errorf("internal id = %s, want %s", id, product.Id)
	}

	if _, err := VerifyProductOwnership(db, rdb, product.PublicId, other.Id.String()); err == nil || err.StatusCode != http.StatusForbidden {
		t.Errorf("other admin: err = %+v, want 403", err)
	}
	if _, err := VerifyProductOwnership(db, rdb, product.PublicId, "not-a-uuid"); err == nil || err.StatusCode != http.StatusUnauthorized {
		t.Errorf("bad admin id: err = %+v, want 401", err)
	}
	if _, err := VerifyProductOwnership(db, rdb, "prod_missing", owner.Id.String()); err == nil || err.StatusCode != http.StatusNotFound {
		t.Errorf("unknown product: err = %+v, want 404", err)
	}
}

func TestVerifyOwnerShip_Store(t *testing.T) {
	db := testutil.NewDB(t)
	owner := testutil.SeedAdmin(t, db)
	store := testutil.SeedStore(t, db, owner.Id)

	if err := VerifyOwnerShip(db, store.ID, owner.Id); err != nil {
		t.Errorf("owner rejected: %s", err.Message)
	}
	if err := VerifyOwnerShip(db, store.ID, uuid.New()); err == nil || err.StatusCode != http.StatusForbidden {
		t.Errorf("stranger: err = %+v, want 403", err)
	}
}

func TestValidateProductId_StatusCodes(t *testing.T) {
	db := testutil.NewDB(t)
	mr, rdb := testutil.NewRedis(t)
	admin := testutil.SeedAdmin(t, db)
	store := testutil.SeedStore(t, db, admin.Id)
	product := testutil.SeedProduct(t, db, store.ID, "Ankara bag", 15000, 5)

	if _, err := ValidateProductId(db, rdb, "prod_missing"); err == nil || err.StatusCode != http.StatusNotFound {
		t.Errorf("unknown product: err = %+v, want 404", err)
	}

	// First call reads the DB and caches; second call is served from Redis.
	for i := 0; i < 2; i++ {
		id, err := ValidateProductId(db, rdb, product.PublicId)
		if err != nil || *id != product.Id {
			t.Fatalf("call %d: id=%v err=%+v", i+1, id, err)
		}
	}

	mr.Close()
	if _, err := ValidateProductId(db, rdb, product.PublicId); err == nil || err.StatusCode != http.StatusInternalServerError {
		t.Errorf("redis down: err = %+v, want 500", err)
	}
}

func TestGetInternalStoreId_UnknownStore(t *testing.T) {
	db := testutil.NewDB(t)
	_, rdb := testutil.NewRedis(t)

	if _, err := GetInternalStoreId(rdb, db, "str-missing"); err == nil || err.StatusCode != http.StatusNotFound {
		t.Errorf("err = %+v, want 404", err)
	}
}
