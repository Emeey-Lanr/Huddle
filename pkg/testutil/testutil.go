// Package testutil provides a real Postgres database and an in-memory Redis
// for tests. It is only imported from _test.go files.
//
// The database comes from TEST_DATABASE_URL, or else from the DB_* values in
// .env with the database name replaced by "huddle_test". The dev database is
// never used. Each test gets its own schema, built from db/migrations and
// dropped afterwards. If Postgres is unreachable, DB tests are skipped.
package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"huddle/pkg/model"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func baseDSN() string {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}
	env, _ := godotenv.Read(filepath.Join(repoRoot(), ".env"))
	get := func(k, def string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		if v := strings.TrimSpace(env[k]); v != "" {
			return v
		}
		return def
	}
	return fmt.Sprintf("host=%s user=%s password=%s dbname=huddle_test port=%s sslmode=disable",
		get("DB_HOST", "localhost"), get("DB_USER", "postgres"), get("DB_PASSWORD", ""), get("DB_PORT", "5432"))
}

func open(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
		TranslateError: true,
		Logger:         logger.Default.LogMode(logger.Silent),
	})
}

func closeDB(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.Close()
	}
}

// NewDB returns a connection to a fresh schema with all migrations applied.
func NewDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := baseDSN()

	admin, err := open(dsn)
	if err == nil {
		err = admin.Exec("SELECT 1").Error
	}
	if err != nil {
		t.Skipf("Postgres not available for tests (set TEST_DATABASE_URL or create huddle_test): %v", err)
	}

	schemaName := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec("CREATE SCHEMA " + schemaName).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		admin.Exec("DROP SCHEMA " + schemaName + " CASCADE")
		closeDB(admin)
	})

	db, err := open(dsn + " search_path=" + schemaName)
	if err != nil {
		t.Fatalf("open test schema: %v", err)
	}
	t.Cleanup(func() { closeDB(db) })

	Migrate(t, db, "up")
	return db
}

// Migrate applies every migration in db/migrations: "up" in order, or "down"
// in reverse order.
func Migrate(t *testing.T, db *gorm.DB, direction string) {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(repoRoot(), "db", "migrations", "*."+direction+".sql"))
	sort.Strings(files)
	if direction == "down" {
		sort.Sort(sort.Reverse(sort.StringSlice(files)))
	}
	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if err := db.Exec(string(sql)).Error; err != nil {
			t.Fatalf("migration %s: %v", filepath.Base(f), err)
		}
	}
}

func NewRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	return mr, redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func SeedAdmin(t *testing.T, db *gorm.DB) model.Admins {
	t.Helper()
	id := uuid.New()
	admin := model.Admins{Id: id, Name: "Seller", Username: "seller-" + id.String()[:8], Email: id.String()[:8] + "@example.com", Password: "x"}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	return admin
}

func SeedStore(t *testing.T, db *gorm.DB, adminId uuid.UUID) model.Stores {
	t.Helper()
	id := uuid.New()
	store := model.Stores{ID: id, AdminId: adminId, StoreName: "Store " + id.String()[:8], StoreDescription: "A test store", ApiKey: id.String(), StoreId: "str-" + id.String()[:8]}
	if err := db.Create(&store).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}
	return store
}

func SeedProduct(t *testing.T, db *gorm.DB, storeId uuid.UUID, name string, price int64, stock int) model.Products {
	t.Helper()
	id := uuid.New()
	product := model.Products{Id: id, StoreId: storeId, PublicId: "prod_" + id.String()[:8], Name: name, Description: "A test product", Price: decimal.NewFromInt(price), Currency: "NGN", StockQuantity: stock, Active: true}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}
	return product
}
