package testutil

import "testing"

// Every migration must roll back cleanly and apply again.
func TestMigrations_DownThenUp(t *testing.T) {
	db := NewDB(t) // all "up" migrations applied

	Migrate(t, db, "down")
	var tables int64
	db.Raw("SELECT count(*) FROM information_schema.tables WHERE table_schema = current_schema()").Scan(&tables)
	if tables != 0 {
		t.Errorf("%d tables left after all down migrations, want 0", tables)
	}

	Migrate(t, db, "up")
	db.Raw("SELECT count(*) FROM information_schema.tables WHERE table_schema = current_schema()").Scan(&tables)
	if tables != 11 {
		t.Errorf("%d tables after re-applying up migrations, want 11", tables)
	}
}

func TestMigrations_IndexesExist(t *testing.T) {
	db := NewDB(t)
	want := []string{
		"admins_email_unique", "admins_username_unique", "stores_admin_store_name_unique",
		"customers_store_email_unique", "products_store_name_unique", "products_public_id_unique",
		"images_product_id_idx", "orders_store_created_idx", "orders_customer_idx", "orders_pending_expiry_idx",
		"order_items_order_idx", "payments_order_idx", "stores_bachs_account_unique",
		"payments_store_status_idx", "refunds_payment_idx", "refunds_one_active_per_payment",
	}
	for _, name := range want {
		var n int64
		db.Raw("SELECT count(*) FROM pg_indexes WHERE schemaname = current_schema() AND indexname = ?", name).Scan(&n)
		if n != 1 {
			t.Errorf("index %s missing", name)
		}
	}
}
