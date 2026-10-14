package services

import (
	"fmt"
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

// seedCatalog adds 25 products priced ₦1,000..₦25,000; every 5th one is hidden.
func seedCatalog(t *testing.T) (productEnv, model.Stores) {
	t.Helper()
	db := testutil.NewDB(t)
	_, rdb := testutil.NewRedis(t)
	admin := testutil.SeedAdmin(t, db)
	store := testutil.SeedStore(t, db, admin.Id)
	for i := 1; i <= 25; i++ {
		p := testutil.SeedProduct(t, db, store.ID, fmt.Sprintf("Item %02d", i), int64(i*1000), 5)
		if i%5 == 0 {
			db.Model(&model.Products{}).Where("id = ?", p.Id).Update("active", false)
		}
	}
	return productEnv{db: db, rdb: rdb, adminId: admin.Id.String()}, store
}

func list(t *testing.T, e productEnv, q model.ProductQuery) *model.ProductListResponse {
	t.Helper()
	got, err := GetAllProducts(e.db, e.rdb, q, e.adminId)
	if err != nil {
		t.Fatalf("GetAllProducts(%+v): %s %v", q, err.Message, err.Err)
	}
	return got
}

func TestGetAllProducts_DefaultPaging(t *testing.T) {
	e, store := seedCatalog(t)

	got := list(t, e, model.ProductQuery{StoreId: store.StoreId})
	if got.Total != 25 || len(got.Items) != 20 || got.Page != 1 || got.Limit != 20 {
		t.Errorf("total=%d items=%d page=%d limit=%d, want 25/20/1/20", got.Total, len(got.Items), got.Page, got.Limit)
	}

	got = list(t, e, model.ProductQuery{StoreId: store.StoreId, Page: 2})
	if len(got.Items) != 5 {
		t.Errorf("page 2 has %d items, want 5", len(got.Items))
	}

	got = list(t, e, model.ProductQuery{StoreId: store.StoreId, Page: 9})
	if got.Items == nil || len(got.Items) != 0 {
		t.Errorf("page past the end should be an empty list, got %v", got.Items)
	}
}

func TestGetAllProducts_PagesDoNotOverlap(t *testing.T) {
	e, store := seedCatalog(t)
	seen := map[string]bool{}
	for page := 1; page <= 3; page++ {
		for _, p := range list(t, e, model.ProductQuery{StoreId: store.StoreId, Page: page, Limit: 10}).Items {
			if seen[p.PublicId] {
				t.Fatalf("product %s appeared on two pages", p.PublicId)
			}
			seen[p.PublicId] = true
		}
	}
	if len(seen) != 25 {
		t.Errorf("saw %d products across pages, want 25", len(seen))
	}
}

func TestGetAllProducts_LimitIsCapped(t *testing.T) {
	e, store := seedCatalog(t)
	if got := list(t, e, model.ProductQuery{StoreId: store.StoreId, Limit: 1000}); got.Limit != 100 {
		t.Errorf("limit = %d, want capped at 100", got.Limit)
	}
}

func TestGetAllProducts_PriceFilters(t *testing.T) {
	e, store := seedCatalog(t)

	got := list(t, e, model.ProductQuery{StoreId: store.StoreId, MinPrice: 5000, MaxPrice: 8000})
	if got.Total != 4 {
		t.Errorf("₦5,000–₦8,000: total = %d, want 4", got.Total)
	}
	got = list(t, e, model.ProductQuery{StoreId: store.StoreId, MaxPrice: 3000})
	if got.Total != 3 {
		t.Errorf("max ₦3,000: total = %d, want 3", got.Total)
	}

	bad := []model.ProductQuery{
		{StoreId: store.StoreId, MinPrice: 9000, MaxPrice: 1000},
		{StoreId: store.StoreId, MinPrice: -1},
		{StoreId: store.StoreId, Sort: "password"},
		{StoreId: store.StoreId, Order: "sideways"},
	}
	for _, q := range bad {
		if _, err := GetAllProducts(e.db, e.rdb, q, e.adminId); err == nil || err.StatusCode != http.StatusBadRequest {
			t.Errorf("%+v: err = %+v, want 400", q, err)
		}
	}
}

func TestGetAllProducts_ActiveFilter(t *testing.T) {
	e, store := seedCatalog(t)
	yes, no := true, false

	if got := list(t, e, model.ProductQuery{StoreId: store.StoreId, Active: &yes}); got.Total != 20 {
		t.Errorf("active=true: total = %d, want 20", got.Total)
	}
	if got := list(t, e, model.ProductQuery{StoreId: store.StoreId, Active: &no}); got.Total != 5 {
		t.Errorf("active=false: total = %d, want 5", got.Total)
	}
}

func TestGetAllProducts_SortByPrice(t *testing.T) {
	e, store := seedCatalog(t)

	got := list(t, e, model.ProductQuery{StoreId: store.StoreId, Sort: "price", Order: "asc", Limit: 3})
	if got.Items[0].Name != "Item 01" || got.Items[2].Name != "Item 03" {
		t.Errorf("cheapest first: got %s, %s", got.Items[0].Name, got.Items[2].Name)
	}
	got = list(t, e, model.ProductQuery{StoreId: store.StoreId, Sort: "price", Order: "DESC", Limit: 1})
	if got.Items[0].Name != "Item 25" {
		t.Errorf("most expensive first: got %s", got.Items[0].Name)
	}
}

func TestGetAllProducts_OtherAdminForbidden(t *testing.T) {
	e, store := seedCatalog(t)
	stranger := testutil.SeedAdmin(t, e.db)

	if _, err := GetAllProducts(e.db, e.rdb, model.ProductQuery{StoreId: store.StoreId}, stranger.Id.String()); err == nil || err.StatusCode != http.StatusForbidden {
		t.Errorf("err = %+v, want 403", err)
	}
}
