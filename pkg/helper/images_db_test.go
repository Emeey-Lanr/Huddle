package helper

import (
	"errors"
	"testing"

	"huddle/pkg/model"
	"huddle/pkg/testutil"

	"github.com/google/uuid"
)

// fakeCloudinary records deletes and returns err for each one.
func fakeCloudinary(t *testing.T, err error) *[]string {
	t.Helper()
	deleted := &[]string{}
	old := deleteFromCloudinary
	deleteFromCloudinary = func(id string) error {
		*deleted = append(*deleted, id)
		return err
	}
	t.Cleanup(func() { deleteFromCloudinary = old })
	return deleted
}

func TestSaveImagesToDb_Success(t *testing.T) {
	db := testutil.NewDB(t)
	admin := testutil.SeedAdmin(t, db)
	store := testutil.SeedStore(t, db, admin.Id)
	product := testutil.SeedProduct(t, db, store.ID, "Ankara bag", 15000, 5)
	deleted := fakeCloudinary(t, nil)

	err := SaveImagesToDb(db, []string{"img_a", "img_b"}, []string{"https://x/a.png", "https://x/b.png"}, product.Id)
	if err != nil {
		t.Fatalf("SaveImagesToDb: %s %v", err.Message, err.Err)
	}

	var count int64
	db.Model(&model.Images{}).Where("product_id = ?", product.Id).Count(&count)
	if count != 2 {
		t.Errorf("saved %d images, want 2", count)
	}
	if len(*deleted) != 0 {
		t.Errorf("deleted %v from Cloudinary on success", *deleted)
	}
}

// The DB insert fails (product does not exist) but Cloudinary cleanup works:
// the function used to return nil here and report a successful upload.
func TestSaveImagesToDb_DBFailureReturnsErrorAfterCleanup(t *testing.T) {
	db := testutil.NewDB(t)
	deleted := fakeCloudinary(t, nil)

	err := SaveImagesToDb(db, []string{"img_a", "img_b"}, []string{"https://x/a.png", "https://x/b.png"}, uuid.New())
	if err == nil {
		t.Fatal("DB failure reported as success")
	}
	if len(*deleted) != 2 {
		t.Errorf("cleaned up %v, want both uploaded images deleted", *deleted)
	}

	var count int64
	db.Model(&model.Images{}).Count(&count)
	if count != 0 {
		t.Errorf("%d image rows left after failed save, want 0", count)
	}
}

func TestSaveImagesToDb_DBFailureAndCleanupFailure(t *testing.T) {
	db := testutil.NewDB(t)
	fakeCloudinary(t, errors.New("cloudinary down"))

	if err := SaveImagesToDb(db, []string{"img_a"}, []string{"https://x/a.png"}, uuid.New()); err == nil {
		t.Fatal("DB failure reported as success")
	}
}
