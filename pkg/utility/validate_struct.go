package utility

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

var Validate = newValidator()

// newValidator reports fields by their JSON names (e.g. "store_description")
// so clients can match errors to the fields they sent.
func newValidator() *validator.Validate {
	v := validator.New()
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		if name == "" {
			return fld.Name
		}
		return name
	})
	return v
}

// ValidationErrors turns validator errors into a JSON object of
// field -> message. Any other error is returned unchanged.
func ValidationErrors(err error) error {
	var validationErrs validator.ValidationErrors
	if !errors.As(err, &validationErrs) {
		return err
	}

	errorsMap := make(map[string]string)
	for _, e := range validationErrs {
		errorsMap[e.Field()] = validationMessage(e)
	}

	b, _ := json.Marshal(errorsMap)
	return errors.New(string(b))
}

func validationMessage(e validator.FieldError) string {
	field, param := e.Field(), e.Param()

	// Length rules mean characters for text, items for lists, and value for numbers.
	unit := ""
	switch e.Kind() {
	case reflect.String:
		unit = " characters"
	case reflect.Slice, reflect.Array, reflect.Map:
		unit = " items"
	}

	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "min", "gte":
		return fmt.Sprintf("%s must be at least %s%s", field, param, unit)
	case "max", "lte":
		return fmt.Sprintf("%s must be at most %s%s", field, param, unit)
	case "gt":
		return fmt.Sprintf("%s must be more than %s%s", field, param, unit)
	case "lt":
		return fmt.Sprintf("%s must be less than %s%s", field, param, unit)
	case "len":
		return fmt.Sprintf("%s must be exactly %s%s", field, param, unit)
	case "http_url", "url":
		return fmt.Sprintf("%s must be a full http(s) link", field)
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, strings.ReplaceAll(param, " ", ", "))
	case "required_if":
		// param is "OtherField value"
		if parts := strings.Fields(param); len(parts) == 2 {
			return fmt.Sprintf("%s is required for %s", field, parts[1])
		}
		return fmt.Sprintf("%s is required", field)
	default:
		return fmt.Sprintf("%s is invalid (%s)", field, e.Tag())
	}
}
