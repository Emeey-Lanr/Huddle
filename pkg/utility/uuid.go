package utility 

import (
	"github.com/google/uuid"
)

func GenerateUUID () uuid.UUID {
  generatedUUID := uuid.New()
 
  return  generatedUUID
}

func StringToUUID (u string) (uuid.UUID, error){
   id, err :=  uuid.Parse(u)
  return id, err
}

