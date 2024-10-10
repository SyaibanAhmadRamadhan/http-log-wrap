package whttp

import (
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"reflect"
)

func NewValidator() *validator.Validate {
	v := validator.New()
	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		return field.Tag.Get("json")
	})

	v.RegisterValidation("required_if", requiredIf)
	return v
}

func registerTranslations(validate *validator.Validate, trans ut.Translator) {
	validate.RegisterTranslation("required_if", trans, func(ut ut.Translator) error {
		return ut.Add("required_if", "{0} must be filled if {1} has a value.", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("required_if", fe.Field(), fe.Param())
		return t
	})
}

func requiredIf(fl validator.FieldLevel) bool {
	fieldName := fl.Param()
	fieldValue := fl.Field().String()

	otherFieldValue := fl.Parent().FieldByName(fieldName).String()

	if otherFieldValue != "" && fieldValue == "" {
		return false
	}
	return true
}
