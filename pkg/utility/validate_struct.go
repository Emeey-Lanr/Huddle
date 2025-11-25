package utility

import (
	
	"fmt"

	"github.com/go-playground/validator/v10"
)

var Validate = validator.New()


func ValidationErrors (err error)map[string]string {
	 errors := make(map[string]string)
	 for _, e := range err.(validator.ValidationErrors){
		var msg string
		m := &msg
		switch e.Tag() {
		case "required":
			*m = fmt.Sprintf("%s is required", e.Field())
		case "email":
			*m = "Invalid email format"
		case "gte":
			*m = fmt.Sprintf("Password length should be at least %s", e.Param())
		}

		errors[e.Field()] = msg
	 }

	 return errors
}




