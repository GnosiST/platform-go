package errorcode

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"
)

type Code string
type Plane string
type Audience string
type Category string
type RetryPolicy string
type RedactionClass string

const (
	PlaneAdmin    Plane = "admin"
	PlaneApp      Plane = "app"
	PlaneService  Plane = "service"
	PlaneData     Plane = "data"
	PlaneControl  Plane = "control"
	PlaneExternal Plane = "external"

	AudienceOperator Audience = "operator"
	AudienceInternal Audience = "internal"
	AudiencePartner  Audience = "partner"
	AudiencePublic   Audience = "public"

	CategoryAuthorization Category = "authorization"
	CategoryValidation    Category = "validation"
	CategoryNotFound      Category = "not-found"
	CategoryConflict      Category = "conflict"
	CategoryRateCost      Category = "rate-cost"
	CategoryDependency    Category = "dependency"
	CategoryInternal      Category = "internal"

	RetryNever      RetryPolicy = "never"
	RetryAfterDelay RetryPolicy = "after-delay"
	RetryBackoff    RetryPolicy = "backoff"

	RedactionPublicSafe      RedactionClass = "public-safe"
	RedactionGenericOnly     RedactionClass = "generic-only"
	RedactionCorrelationOnly RedactionClass = "correlation-only"
)

type Definition struct {
	Code           Code           `json:"code"`
	Owner          string         `json:"owner"`
	Planes         []Plane        `json:"planes"`
	Audiences      []Audience     `json:"audiences"`
	Category       Category       `json:"category"`
	HTTPStatus     int            `json:"httpStatus"`
	RetryPolicy    RetryPolicy    `json:"retryPolicy"`
	RedactionClass RedactionClass `json:"redactionClass"`
	PublicMessage  string         `json:"publicMessage"`
	IntroducedIn   string         `json:"introducedIn"`
	Deprecated     bool           `json:"deprecated"`
	SunsetAt       string         `json:"sunsetAt,omitempty"`
	ReplacedBy     Code           `json:"replacedBy,omitempty"`
}

var codePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{2,127}$`)

func (definition Definition) Validate() error {
	if !codePattern.MatchString(string(definition.Code)) {
		return fmt.Errorf("invalid code %q", definition.Code)
	}
	if definition.Owner == "" {
		return errors.New("owner is required")
	}
	if len(definition.Planes) == 0 {
		return errors.New("at least one plane is required")
	}
	for _, plane := range definition.Planes {
		if !validPlane(plane) {
			return fmt.Errorf("invalid plane %q", plane)
		}
	}
	if len(definition.Audiences) == 0 {
		return errors.New("at least one audience is required")
	}
	for _, audience := range definition.Audiences {
		if !validAudience(audience) {
			return fmt.Errorf("invalid audience %q", audience)
		}
	}
	if !validCategory(definition.Category) {
		return fmt.Errorf("invalid category %q", definition.Category)
	}
	if definition.HTTPStatus < 400 || definition.HTTPStatus > 599 {
		return fmt.Errorf("invalid HTTP status %d", definition.HTTPStatus)
	}
	if !validRetryPolicy(definition.RetryPolicy) {
		return fmt.Errorf("invalid retry policy %q", definition.RetryPolicy)
	}
	if !validRedactionClass(definition.RedactionClass) {
		return fmt.Errorf("invalid redaction class %q", definition.RedactionClass)
	}
	if definition.PublicMessage == "" {
		return errors.New("public message is required")
	}
	if definition.IntroducedIn == "" {
		return errors.New("introduced version is required")
	}
	if definition.Deprecated {
		if definition.SunsetAt == "" || definition.ReplacedBy == "" {
			return errors.New("deprecated definitions require sunsetAt and replacedBy")
		}
		if definition.ReplacedBy == definition.Code {
			return errors.New("deprecated definition cannot replace itself")
		}
		if _, err := time.Parse("2006-01-02", definition.SunsetAt); err != nil {
			return fmt.Errorf("invalid sunsetAt %q", definition.SunsetAt)
		}
	} else if definition.SunsetAt != "" || definition.ReplacedBy != "" {
		return errors.New("active definitions cannot include deprecation metadata")
	}
	return nil
}

func Lookup(code Code) (Definition, bool) {
	definition, ok := registry[code]
	return cloneDefinition(definition), ok
}

func All() []Definition {
	definitions := make([]Definition, 0, len(registry))
	for _, definition := range registry {
		definitions = append(definitions, cloneDefinition(definition))
	}
	sort.Slice(definitions, func(i, j int) bool { return definitions[i].Code < definitions[j].Code })
	return definitions
}

func New(code Code) error {
	return &platformError{code: registeredOrInternal(code)}
}

func Wrap(code Code, cause error) error {
	return &platformError{code: registeredOrInternal(code), cause: cause}
}

func CodeOf(err error) (Code, bool) {
	var platformErr *platformError
	if !errors.As(err, &platformErr) {
		return "", false
	}
	return platformErr.code, true
}

type platformError struct {
	code  Code
	cause error
}

func (err *platformError) Error() string {
	if err == nil {
		return ""
	}
	return string(err.code)
}

func (err *platformError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.cause
}

func registeredOrInternal(code Code) Code {
	if _, ok := registry[code]; ok {
		return code
	}
	return CodeInternal
}

func validateDefinitions(definitions []Definition) error {
	byCode := make(map[Code]Definition, len(definitions))
	for _, definition := range definitions {
		if err := definition.Validate(); err != nil {
			return fmt.Errorf("definition %q: %w", definition.Code, err)
		}
		if _, exists := byCode[definition.Code]; exists {
			return fmt.Errorf("duplicate code %q", definition.Code)
		}
		byCode[definition.Code] = definition
	}
	for _, definition := range definitions {
		if !definition.Deprecated {
			continue
		}
		if _, ok := byCode[definition.ReplacedBy]; !ok {
			return fmt.Errorf("definition %q has unknown replacement %q", definition.Code, definition.ReplacedBy)
		}
		seen := map[Code]struct{}{definition.Code: {}}
		cursor := definition
		for cursor.Deprecated {
			if _, exists := seen[cursor.ReplacedBy]; exists {
				return fmt.Errorf("definition %q has a replacement cycle", definition.Code)
			}
			seen[cursor.ReplacedBy] = struct{}{}
			cursor = byCode[cursor.ReplacedBy]
		}
	}
	return nil
}

func cloneDefinition(definition Definition) Definition {
	definition.Planes = append([]Plane(nil), definition.Planes...)
	definition.Audiences = append([]Audience(nil), definition.Audiences...)
	return definition
}

func validPlane(value Plane) bool {
	switch value {
	case PlaneAdmin, PlaneApp, PlaneService, PlaneData, PlaneControl, PlaneExternal:
		return true
	default:
		return false
	}
}

func validAudience(value Audience) bool {
	switch value {
	case AudienceOperator, AudienceInternal, AudiencePartner, AudiencePublic:
		return true
	default:
		return false
	}
}

func validCategory(value Category) bool {
	switch value {
	case CategoryAuthorization, CategoryValidation, CategoryNotFound, CategoryConflict, CategoryRateCost, CategoryDependency, CategoryInternal:
		return true
	default:
		return false
	}
}

func validRetryPolicy(value RetryPolicy) bool {
	switch value {
	case RetryNever, RetryAfterDelay, RetryBackoff:
		return true
	default:
		return false
	}
}

func validRedactionClass(value RedactionClass) bool {
	switch value {
	case RedactionPublicSafe, RedactionGenericOnly, RedactionCorrelationOnly:
		return true
	default:
		return false
	}
}
