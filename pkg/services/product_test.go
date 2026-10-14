package services

import (
	"net/http"
	"testing"

	"huddle/pkg/model"
	"huddle/pkg/testutil"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type productEnv struct {
	db      *gorm.DB
	rdb     *redis.Client
	adminId string
	product model.Products // ₦15,000, 10 in stock
}

func setupProduct(t *testing.T) productEnv {
	t.Helper()
	db := testutil.NewDB(t)
	_, rdb := testutil.NewRedis(t)
	admin := testutil.SeedAdmin(t, db)
	store := testutil.SeedStore(t, db, admin.Id)
	return productEnv{
		db:      db,
		rdb:     rdb,
		adminId: admin.Id.String(),
		product: testutil.SeedProduct(t, db, store.ID, "Ankara bag", 15000, 10),
	}
}

func intPtr(i int) *int { return &i }

func TestUpdateProduct_PriceOnlyKeepsStock(t *testing.T) {
	e := setupProduct(t)
	price := decimal.NewFromInt(18000)

	got, err := UpdateProduct(e.db, e.rdb, e.product.PublicId, e.adminId, model.ProductUpdate{Price: &price})
	if err != nil {
		t.Fatalf("UpdateProduct: %s %v", err.Message, err.Err)
	}
	if !got.Price.Equal(price) {
		t.Errorf("price = %s, want 18000", got.Price)
	}
	if got.StockQuantity != 10 {
		t.Errorf("stock = %d, want 10 (unchanged)", got.StockQuantity)
	}
}

func TestUpdateProduct_EmptyRequestChangesNothing(t *testing.T) {
	e := setupProduct(t)

	got, err := UpdateProduct(e.db, e.rdb, e.product.PublicId, e.adminId, model.ProductUpdate{})
	if err != nil {
		t.Fatalf("UpdateProduct: %s", err.Message)
	}
	if got.Name != "Ankara bag" || !got.Price.Equal(e.product.Price) || got.StockQuantity != 10 || !got.Active {
		t.Errorf("product changed: %+v", got)
	}
}

func TestUpdateProduct_StockCanBeSetToZero(t *testing.T) {
	e := setupProduct(t)

	got, err := UpdateProduct(e.db, e.rdb, e.product.PublicId, e.adminId, model.ProductUpdate{StockQuantity: intPtr(0)})
	if err != nil {
		t.Fatalf("UpdateProduct: %s", err.Message)
	}
	if got.StockQuantity != 0 {
		t.Errorf("stock = %d, want 0", got.StockQuantity)
	}
}

func TestUpdateProduct_RejectsNegativeValues(t *testing.T) {
	e := setupProduct(t)
	negative := decimal.NewFromInt(-1)

	if _, err := UpdateProduct(e.db, e.rdb, e.product.PublicId, e.adminId, model.ProductUpdate{StockQuantity: intPtr(-5)}); err == nil || err.StatusCode != http.StatusBadRequest {
		t.Errorf("negative stock: err = %+v, want 400", err)
	}
	if _, err := UpdateProduct(e.db, e.rdb, e.product.PublicId, e.adminId, model.ProductUpdate{Price: &negative}); err == nil || err.StatusCode != http.StatusBadRequest {
		t.Errorf("negative price: err = %+v, want 400", err)
	}
}

func TestUpdateProduct_OtherAdminForbidden(t *testing.T) {
	e := setupProduct(t)
	stranger := testutil.SeedAdmin(t, e.db)

	_, err := UpdateProduct(e.db, e.rdb, e.product.PublicId, stranger.Id.String(), model.ProductUpdate{StockQuantity: intPtr(1)})
	if err == nil || err.StatusCode != http.StatusForbidden {
		t.Errorf("err = %+v, want 403", err)
	}
}
