// Package sensitivereveal implements the server-side step-up state machine for
// revealing one sensitive field. It never handles the field's plaintext value;
// grants are short lived, single use, and bound to an exact reveal scope.
package sensitivereveal
