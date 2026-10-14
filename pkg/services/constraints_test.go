package services

import (
	"net/http"
	"sync"
	"testing"

	"huddle/pkg/model"
	"huddle/pkg/testutil"
	"huddle/pkg/utility"

	"github.com/shopspring/decimal"
)

// These tests change letter case so the app's own "already exists" checks
// (which are case-sensitive) pass, and the database rule has to catch it.

func TestAdminSignup_DuplicateEmailDifferentCase(t *testing.T) {
	t.Setenv("SECRET_KEY", "test-secret")
	db := testutil.NewDB(t)

	if _, err := AdminSignup(db, model.Admins{Name: "Ada", Username: "ada", Email: "ada@example.com", Password: "secret123"}); err != nil {
		t.Fatalf("first signup: %s %v", err.Message, err.Err)
	}
	_, err := AdminSignup(db, model.Admins{Name: "Ada 2", Username: "ada2", Email: "ADA@example.com", Password: "secret123"})
	if err == nil || err.StatusCode != http.StatusConflict {
		t.Errorf("same email, different case: err = %+v, want 409", err)
	}
	_, err = AdminSignup(db, model.Admins{Name: "Ada 3", Username: "ADA", Email: "other@example.com", Password: "secret123"})
	if err == nil || err.StatusCode != http.StatusConflict {
		t.Errorf("same username, different case: err = %+v, want 409", err)
	}
}

// Ten simultaneous signups with one email: exactly one may succeed.
func TestAdminSignup_ConcurrentDuplicates(t *testing.T) {
	t.Setenv("SECRET_KEY", "test-secret")
	db := testutil.NewDB(t)

	const n = 10
	var wg sync.WaitGroup
	results := make(chan *utility.AppError, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := AdminSignup(db, model.Admins{Name: "Ada", Username: "ada", Email: "ada@example.com", Password: "secret123"})
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	ok, conflicts := 0, 0
	for err := range results {
		switch {
		case err == nil:
			ok++
		case err.StatusCode == http.StatusConflict:
			conflicts++
		default:
			t.Errorf("unexpected error: %s %v", err.Message, err.Err)
		}
	}
	var rows int64
	db.Model(&model.Admins{}).Count(&rows)
	if ok != 1 || conflicts != n-1 || rows != 1 {
		t.Errorf("ok=%d conflicts=%d rows=%d, want 1/%d/1", ok, conflicts, rows, n-1)
	}
}

func TestCreateStore_DuplicateNameDifferentCase(t *testing.T) {
	db := testutil.NewDB(t)
	admin := testutil.SeedAdmin(t, db)

	if _, _, err := CreateStore(db, model.Stores{StoreName: "Chioma's Bags", StoreDescription: "Handmade bags"}, admin.Id, admin.Email); err != nil {
		t.Fatalf("first store: %s %v", err.Message, err.Err)
	}
	_, _, err := CreateStore(db, model.Stores{StoreName: "CHIOMA'S BAGS", StoreDescription: "Handmade bags"}, admin.Id, admin.Email)
	if err == nil || err.StatusCode != http.StatusConflict {
		t.Errorf("err = %+v, want 409", err)
	}

	// Another admin may use the same name.
	other := testutil.SeedAdmin(t, db)
	if _, _, err := CreateStore(db, model.Stores{StoreName: "Chioma's Bags", StoreDescription: "Handmade bags"}, other.Id, other.Email); err != nil {
		t.Errorf("other admin, same name: %s", err.Message)
	}
}

func TestAddProduct_DuplicateNameDifferentCase(t *testing.T) {
	db := testutil.NewDB(t)
	_, rdb := testutil.NewRedis(t)
	admin := testutil.SeedAdmin(t, db)
	store := testutil.SeedStore(t, db, admin.Id)
	req := model.Products{Name: "Ankara bag", Description: "Handmade ankara bag", Price: decimal.NewFromInt(15000), Currency: "NGN", StockQuantity: 5}

	if _, err := AddProduct(req, db, rdb, admin.Id.String(), store.StoreId); err != nil {
		t.Fatalf("first add: %s", err.Message)
	}
	req.Name = "ANKARA BAG"
	if _, err := AddProduct(req, db, rdb, admin.Id.String(), store.StoreId); err == nil || err.StatusCode != http.StatusConflict {
		t.Errorf("err = %+v, want 409", err)
	}
}

func TestUpdateProduct_RenameToExistingNameConflict(t *testing.T) {
	e := setupProduct(t)
	store := model.Stores{}
	e.db.Where("admin_id = ?", e.adminId).First(&store)
	testutil.SeedProduct(t, e.db, store.ID, "Fila cap", 5000, 3)

	if _, err := UpdateProduct(e.db, e.rdb, e.product.PublicId, e.adminId, model.ProductUpdate{Name: "fila cap"}); err == nil || err.StatusCode != http.StatusConflict {
		t.Errorf("err = %+v, want 409", err)
	}
}
