package errorcode

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestBuiltinsAreUniqueCompleteAndStable(t *testing.T) {
	definitions := All()
	if got, want := len(definitions), 117; got != want {
		t.Fatalf("len(All()) = %d, want %d", got, want)
	}
	seen := map[Code]struct{}{}
	for _, definition := range definitions {
		if _, exists := seen[definition.Code]; exists {
			t.Fatalf("duplicate code %q", definition.Code)
		}
		seen[definition.Code] = struct{}{}
		if err := definition.Validate(); err != nil {
			t.Fatalf("definition %q: %v", definition.Code, err)
		}
	}
	for _, code := range []Code{CodeInternal, CodeAdminForbidden, CodeAppForbidden, CodeRateLimited, CodeServiceObjectCostLimit} {
		if _, ok := Lookup(code); !ok {
			t.Fatalf("Lookup(%q) = false", code)
		}
	}
}

func TestPublicErrorDoesNotRenderWrappedCause(t *testing.T) {
	cause := errors.New("password=marker physical_table=users")
	err := Wrap(CodeInternal, cause)
	if strings.Contains(err.Error(), "marker") || strings.Contains(err.Error(), "physical_table") {
		t.Fatalf("public error leaked cause: %v", err)
	}
	if code, ok := CodeOf(err); !ok || code != CodeInternal {
		t.Fatalf("CodeOf() = %q, %t", code, ok)
	}
	if !errors.Is(err, cause) {
		t.Fatal("wrapped cause is not available to internal errors.Is checks")
	}
}

func TestNewFallsBackToRegisteredInternalCode(t *testing.T) {
	err := New("UNREGISTERED_PRIVATE_CODE")
	if code, ok := CodeOf(err); !ok || code != CodeInternal {
		t.Fatalf("CodeOf(New(unknown)) = %q, %t", code, ok)
	}
	if err.Error() != string(CodeInternal) {
		t.Fatalf("New(unknown).Error() = %q", err.Error())
	}
}

func TestAllReturnsSortedDefensiveCopies(t *testing.T) {
	first := All()
	for i := 1; i < len(first); i++ {
		if first[i-1].Code >= first[i].Code {
			t.Fatalf("definitions are not sorted at %q", first[i].Code)
		}
	}
	first[0].Planes[0] = PlaneExternal
	first[0].Audiences[0] = AudiencePartner
	if reflect.DeepEqual(first, All()) {
		t.Fatal("All returned registry-owned slices")
	}
}

func TestDefinitionValidateRejectsInvalidDefinitions(t *testing.T) {
	valid, ok := Lookup(CodeAdminForbidden)
	if !ok {
		t.Fatal("ADMIN_FORBIDDEN is missing")
	}
	tests := []struct {
		name string
		edit func(*Definition)
	}{
		{name: "code", edit: func(definition *Definition) { definition.Code = "bad-code" }},
		{name: "owner", edit: func(definition *Definition) { definition.Owner = "" }},
		{name: "plane", edit: func(definition *Definition) { definition.Planes = []Plane{"unknown"} }},
		{name: "audience", edit: func(definition *Definition) { definition.Audiences = []Audience{"unknown"} }},
		{name: "category", edit: func(definition *Definition) { definition.Category = "unknown" }},
		{name: "status", edit: func(definition *Definition) { definition.HTTPStatus = 399 }},
		{name: "retry", edit: func(definition *Definition) { definition.RetryPolicy = "unknown" }},
		{name: "redaction", edit: func(definition *Definition) { definition.RedactionClass = "unknown" }},
		{name: "message", edit: func(definition *Definition) { definition.PublicMessage = "" }},
		{name: "version", edit: func(definition *Definition) { definition.IntroducedIn = "" }},
		{name: "partial deprecation", edit: func(definition *Definition) { definition.Deprecated = true; definition.ReplacedBy = CodeInternal }},
		{name: "metadata without deprecation", edit: func(definition *Definition) { definition.SunsetAt = "2027-01-01" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			definition := valid
			test.edit(&definition)
			if err := definition.Validate(); err == nil {
				t.Fatalf("Validate() accepted invalid %s", test.name)
			}
		})
	}
}

func TestValidateDefinitionsRejectsDuplicateAndUnknownReplacement(t *testing.T) {
	definition, _ := Lookup(CodeAdminForbidden)
	if err := validateDefinitions([]Definition{definition, definition}); err == nil {
		t.Fatal("validateDefinitions accepted duplicate code")
	}
	definition.Deprecated = true
	definition.SunsetAt = "2027-01-01"
	definition.ReplacedBy = "DOES_NOT_EXIST"
	if err := validateDefinitions([]Definition{definition}); err == nil {
		t.Fatal("validateDefinitions accepted unknown replacement")
	}
}

func TestCurrentConflictDecisionsAreCanonical(t *testing.T) {
	tests := map[Code]struct {
		status  int
		message string
	}{
		CodeAdminFileOpenFailed:            {status: 500, message: "file open failed"},
		CodeAdminFileUploadOpenFailed:      {status: 400, message: "open uploaded file failed"},
		CodeAppFileOpenFailed:              {status: 500, message: "file open failed"},
		CodeAppFileUploadOpenFailed:        {status: 400, message: "open uploaded file failed"},
		CodeAuthInvalidCredentials:         {status: 401, message: "invalid credentials"},
		CodeAppPhoneInvalidRequest:         {status: 400, message: "invalid app phone request"},
		CodeAdminResourceLifecycleConflict: {status: 409, message: "admin resource lifecycle conflict"},
		CodeAdminResourceInvalidRecord:     {status: 400, message: "invalid admin resource record"},
	}
	for code, want := range tests {
		definition, ok := Lookup(code)
		if !ok {
			t.Fatalf("Lookup(%q) = false", code)
		}
		if definition.HTTPStatus != want.status || definition.PublicMessage != want.message {
			t.Fatalf("Lookup(%q) = status %d message %q", code, definition.HTTPStatus, definition.PublicMessage)
		}
	}
}

func TestBuiltinRetryAndRedactionMetadataIsSemantic(t *testing.T) {
	tests := []struct {
		code      Code
		retry     RetryPolicy
		redaction RedactionClass
	}{
		{code: CodeAdminFileSaveFailed, retry: RetryNever, redaction: RedactionCorrelationOnly},
		{code: CodeAppFileRollbackFailed, retry: RetryNever, redaction: RedactionCorrelationOnly},
		{code: CodeAuthSessionIssueFailed, retry: RetryNever, redaction: RedactionCorrelationOnly},
		{code: CodeAppAuthSessionRevokeFailed, retry: RetryNever, redaction: RedactionCorrelationOnly},
		{code: CodeAuthIdentityBindingFailed, retry: RetryNever, redaction: RedactionCorrelationOnly},
		{code: CodeAuthProviderResolveFailed, retry: RetryBackoff, redaction: RedactionGenericOnly},
		{code: CodeRateLimitUnavailable, retry: RetryBackoff, redaction: RedactionGenericOnly},
		{code: CodeAuthProviderResolverNotConfigured, retry: RetryNever, redaction: RedactionGenericOnly},
		{code: CodeAppRouteHandlerNotConfigured, retry: RetryNever, redaction: RedactionGenericOnly},
		{code: CodeRateLimited, retry: RetryAfterDelay, redaction: RedactionPublicSafe},
		{code: CodeInternal, retry: RetryNever, redaction: RedactionCorrelationOnly},
	}
	for _, test := range tests {
		definition, ok := Lookup(test.code)
		if !ok {
			t.Fatalf("Lookup(%q) = false", test.code)
		}
		if definition.RetryPolicy != test.retry || definition.RedactionClass != test.redaction {
			t.Errorf("Lookup(%q) metadata = %q/%q, want %q/%q", test.code, definition.RetryPolicy, definition.RedactionClass, test.retry, test.redaction)
		}
	}
}
