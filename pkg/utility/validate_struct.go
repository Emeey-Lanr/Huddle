package utility

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
)

var Validate = validator.New()


func ValidationErrors (err error)(error) {
	 errorsMap := make(map[string]string)

	 for _, e := range err.(validator.ValidationErrors){
		var msg string
		
		
		switch e.Tag() {
		case "required":
			msg = fmt.Sprintf("%s is required", e.Field())
		case "email":
			msg = "Invalid email format"
		case "min":
			msg = fmt.Sprintf("Password length should be at least %s characters", e.Param())
		}

		
		errorsMap[e.Field()] = msg
	 }

	 b, _ := json.Marshal(errorsMap)
	 
	  return errors.New(string(b))
}




