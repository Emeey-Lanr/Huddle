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
	if _, err := VerifyProductOwnership(db, rdb, "prod_missing", owner.Id.String()); err == nil {
		t.Error("unknown product accepted")
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
