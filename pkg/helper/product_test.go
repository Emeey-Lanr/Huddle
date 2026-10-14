package helper

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.White)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// fileHeaders builds real multipart file headers so Open() works.
func fileHeaders(t *testing.T, files ...[]byte) []*multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for i, f := range files {
		w, _ := mw.CreateFormFile("image", "f"+string(rune('a'+i))+".png")
		w.Write(f)
	}
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if err := req.ParseMultipartForm(32 << 20); err != nil {
		t.Fatal(err)
	}
	return req.MultipartForm.File["image"]
}

func TestValidateImages_Valid(t *testing.T) {
	if err := ValidateImages(fileHeaders(t, pngBytes(t), pngBytes(t))); err != nil {
		t.Errorf("valid images rejected: %s", err.Message)
	}
}

func TestValidateImages_Count(t *testing.T) {
	if err := ValidateImages(nil); err == nil || err.StatusCode != http.StatusBadRequest {
		t.Error("zero images accepted")
	}
	p := pngBytes(t)
	if err := ValidateImages(fileHeaders(t, p, p, p, p, p, p)); err == nil {
		t.Error("six images accepted, max is 5")
	}
}

func TestValidateImages_NotAnImage(t *testing.T) {
	if err := ValidateImages(fileHeaders(t, []byte("<?php echo 'hi'; ?>"))); err == nil {
		t.Error("non-image file accepted")
	}
}

func TestValidateImages_TooLarge(t *testing.T) {
	headers := fileHeaders(t, pngBytes(t))
	headers[0].Size = 5*1024*1024 + 1
	if err := ValidateImages(headers); err == nil {
		t.Error("file over 5MB accepted")
	}
}
