package utility

import (
	
	"fmt"

	"github.com/go-playground/validator/v10"
)

var Validate = validator.New()


func ValidationErrors (err error)(map[string]string, string) {
	 errors := make(map[string]string)
	 var ms *string
	 for _, e := range err.(validator.ValidationErrors){
		var msg *string
		
		
		switch e.Tag() {
		case "required":
			*msg = fmt.Sprintf("%s is required", e.Field())
		case "email":
			*msg = "Invalid email format"
		case "gte":
			*msg = fmt.Sprintf("Password length should be at least %s", e.Param())
		}

		*ms += fmt.Sprintf("-%v",*msg)
		errors[e.Field()] = *msg
	 }

	 return errors, *ms
}




