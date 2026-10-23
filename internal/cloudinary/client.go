package cloudinary

import (
	"context"
	"mime/multipart"
	"os"
	"sync"
	"time"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

const (
	// Uploads come from sellers on slow mobile data; deletes are quick.
	uploadTimeout = 60 * time.Second
	deleteTimeout = 20 * time.Second
)

// client is created once, on first use, from CLOUDINARY_URL; a bad URL is
// remembered and returned to every caller.
var client = sync.OnceValues(func() (*cloudinary.Cloudinary, error) {
	cld, err := cloudinary.NewFromURL(os.Getenv("CLOUDINARY_URL"))
	if err != nil {
		return nil, err
	}
	cld.Config.URL.Secure = true
	return cld, nil
})

func UploadImageToCloudinary(image *multipart.FileHeader, id string) (*uploader.UploadResult, error) {
	cld, err := client()
	if err != nil {
		return nil, err
	}

	file, err := image.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	ctx, cancel := context.WithTimeout(context.Background(), uploadTimeout)
	defer cancel()
	return cld.Upload.Upload(ctx, file, uploader.UploadParams{
		PublicID:       id,
		UniqueFilename: api.Bool(false),
		Overwrite:      api.Bool(true),
	})
}

func DeleteImageFromCloudinary(id string) error {
	cld, err := client()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), deleteTimeout)
	defer cancel()
	_, err = cld.Upload.Destroy(ctx, uploader.DestroyParams{PublicID: id})
	return err
}
