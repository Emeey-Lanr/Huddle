package utility 

import (
	"github.com/google/uuid"
)

func GenerateUUID () uuid.UUID {
  generatedUUID := uuid.New()
 
  return  generatedUUID
}