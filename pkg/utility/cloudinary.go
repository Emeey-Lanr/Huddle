package utility

import (
	"context"
	"mime/multipart"
	"os"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)



func CloudinaryInit ()(*cloudinary.Cloudinary, error ){
	 cldinary, err := cloudinary.NewFromParams (
   os.Getenv("CLOUDINARY_API_NAME"),
    os.Getenv("CLOUDINARY_API_KEY"),
	 os.Getenv("CLOUDINARY_API_SECRET"),
 )
 if err != nil{
	return nil, err
 }
return cldinary, err

}


func UploadImageToCloudinary (image *multipart.FileHeader, id string)(*uploader.UploadResult, error){
	cloudinary, err := CloudinaryInit()
	if err != nil{
		return nil,  err
	}

	uploadedImg, err := cloudinary.Upload.Upload(context.Background(),image,uploader.UploadParams{
		PublicID:id,
		Folder: "huddle-products",
	} )
	if err != nil{
		return nil, err
	}

return uploadedImg, nil
   	
}

func DeleteImageFromCloudinary (id string)(error){
	cloudinary, err := CloudinaryInit()
	if err != nil{
		return err
	}

	_, err = cloudinary.Upload.Destroy(context.Background(), uploader.DestroyParams{
		PublicID: id,
	})
	if err != nil{
		return err
	}

	return nil
}