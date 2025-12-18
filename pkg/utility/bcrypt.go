package utility

import (
	"golang.org/x/crypto/bcrypt"
)


func HashPassword (password string) (string, error){
	
 hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

 if err != nil{
  return "", err
 }

   return  string(hashedPassword), nil
}



func ComparePassword(hashedPassw, NotHashedPassw string) error {
   err := bcrypt.CompareHashAndPassword([]byte(hashedPassw), []byte(NotHashedPassw))
    
   if err != nil {
    return err
   } 

   return nil

 }