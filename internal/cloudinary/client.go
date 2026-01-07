package cloudinary

import (
	"context"
	"mime/multipart"
	"os"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api"

	// "github.com/cloudinary/cloudinary-go/v2/api/admin"

	"fmt"

	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)



func CloudinaryInit ()(*cloudinary.Cloudinary,context.Context, error ){
 cld, err := cloudinary.NewFromURL(os.Getenv("CLOUDINARY_URL"))
 if err != nil{
	return nil, nil, err
 }

 cld.Config.URL.Secure = true
ctx := context.Background()
return cld, ctx, err

}


func UploadImageToCloudinary (image *multipart.FileHeader, id string)(*uploader.UploadResult, error){
	cloudinary, ctx, err := CloudinaryInit()
	if err != nil{
		return nil,  err
	}

  file, err := image.Open()
  if err != nil{
	return nil, err
  }
  defer file.Close()

resp, err := cloudinary.Upload.Upload(ctx, file,uploader.UploadParams{
		PublicID:id,
	    UniqueFilename: api.Bool(false),
		Overwrite:      api.Bool(true),
	} )
	if err != nil{
		return nil, err
	}

fmt.Println(resp)
return resp, nil
}

func DeleteImageFromCloudinary (id string)(error){
	cloudinary, ctx, err := CloudinaryInit()
	if err != nil{
		return err
	}

	_, err = cloudinary.Upload.Destroy(ctx, uploader.DestroyParams{
		PublicID: id,
	})
	if err != nil{
		return err
	}

	return nil
}