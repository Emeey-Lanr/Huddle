package utility
import(
		"crypto/rand"
	"encoding/hex"
)

func GenerateAPIKey ()(string, error){
	bts := make([]byte, 34)

	  _, err := rand.Read(bts)
	  if err != nil {
		return "", err
	  }

	  return hex.EncodeToString(bts), nil
}