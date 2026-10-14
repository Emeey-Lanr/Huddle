package model

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin/binding"
)

func TestProductQuery_BindsQueryString(t *testing.T) {
	req := httptest.NewRequest("GET", "/product/products?store_id=str-1&page=2&limit=5&min_price=1500.5&max_price=9000&active=false&sort=price&order=asc", nil)

	var q ProductQuery
	if err := binding.Query.Bind(req, &q); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if q.StoreId != "str-1" || q.Page != 2 || q.Limit != 5 || q.Sort != "price" || q.Order != "asc" {
		t.Errorf("unexpected binding: %+v", q)
	}
	if q.MinPrice != 1500.5 || q.MaxPrice != 9000 {
		t.Errorf("prices = %v..%v, want 1500.5..9000", q.MinPrice, q.MaxPrice)
	}
	if q.Active == nil || *q.Active {
		t.Errorf("active = %v, want false", q.Active)
	}
}
