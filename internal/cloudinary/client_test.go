package cloudinary

import "testing"

func TestClient_CreatedOnce(t *testing.T) {
	t.Setenv("CLOUDINARY_URL", "cloudinary://key:secret@demo-cloud")

	first, err := client()
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	second, _ := client()
	if first != second {
		t.Error("client() built a new Cloudinary client on the second call")
	}
	if !first.Config.URL.Secure || first.Config.Cloud.CloudName != "demo-cloud" {
		t.Errorf("config: secure=%v cloud=%q", first.Config.URL.Secure, first.Config.Cloud.CloudName)
	}
}
