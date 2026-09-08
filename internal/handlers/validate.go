package handlers

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// The `validate` tags on the request DTOs described the intended contract but
// nothing read them: there was no validator in the module at all. Every tag was
// documentation. A proposal could arrive with no agent, no venue and a slippage
// of -5 and be persisted unchallenged.
//
// One shared instance: constructing a validator per request re-parses struct
// tags via reflection each time, and the library is designed to be reused.
var validate = validator.New(validator.WithRequiredStructEnabled())

// validateStruct returns a human-readable message naming every field that
// failed, or an empty string when the payload is acceptable.
//
// All failures are reported together rather than stopping at the first, so a
// caller fixing a malformed request does not have to discover the problems one
// round-trip at a time.
func validateStruct(payload any) string {
	err := validate.Struct(payload)
	if err == nil {
		return ""
	}

	var invalid *validator.InvalidValidationError
	if ok := asInvalid(err, &invalid); ok {
		// Programmer error — a non-struct was passed in. Surface it rather than
		// reporting the request as invalid.
		return "internal validation error"
	}

	var msgs []string
	for _, fe := range err.(validator.ValidationErrors) {
		msgs = append(msgs, describe(fe))
	}
	return strings.Join(msgs, "; ")
}

func asInvalid(err error, target **validator.InvalidValidationError) bool {
	v, ok := err.(*validator.InvalidValidationError)
	if ok {
		*target = v
	}
	return ok
}

// describe turns a tag failure into something an API consumer can act on.
// The library's default message is the tag name alone, which does not say what
// the caller should have sent.
func describe(fe validator.FieldError) string {
	field := jsonName(fe)
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "gte":
		return fmt.Sprintf("%s must be at least %s", field, fe.Param())
	case "lte":
		return fmt.Sprintf("%s must be at most %s", field, fe.Param())
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, fe.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, strings.ReplaceAll(fe.Param(), " ", ", "))
	default:
		return fmt.Sprintf("%s is invalid (%s)", field, fe.Tag())
	}
}

// jsonName reports the field as the caller sent it, not as Go spells it.
// Telling an API consumer that "ProjectedSlippage" is invalid is unhelpful when
// they sent "projected_slippage".
func jsonName(fe validator.FieldError) string {
	name := fe.Field()
	return toSnake(name)
}

// toSnake converts a Go field name to its JSON spelling.
//
// Runs of capitals are kept together so that acronyms survive: AgentID becomes
// agent_id, not agent_i_d. An underscore is inserted before a capital only when
// it starts a new word — that is, when the previous character was lowercase, or
// when the capital is followed by a lowercase one (as the D in IDToken).
func toSnake(s string) string {
	runes := []rune(s)
	var b strings.Builder

	for i, r := range runes {
		isUpper := r >= 'A' && r <= 'Z'
		if isUpper && i > 0 {
			prevLower := runes[i-1] >= 'a' && runes[i-1] <= 'z'
			nextLower := i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z'
			if prevLower || nextLower {
				b.WriteByte('_')
			}
		}
		if isUpper {
			b.WriteRune(r + 32)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
