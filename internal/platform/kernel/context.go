package kernel

import "context"

type ActorKind string

const (
	ActorKindUser   ActorKind = "user"
	ActorKindSystem ActorKind = "system"
)

type Actor struct {
	ID       int64
	Username string
	Kind     ActorKind
}

func (a Actor) Empty() bool {
	return a.ID == 0 && a.Username == "" && a.Kind == ""
}

type TenantScope struct {
	TenantID     int64
	PlatformWide bool
}

func (s TenantScope) Empty() bool {
	return s.TenantID == 0 && !s.PlatformWide
}

type PermissionIntent struct {
	Code   string
	Action string
}

func (p PermissionIntent) Empty() bool {
	return p.Code == "" || p.Action == ""
}

type ExecutionContext struct {
	Context          context.Context
	Actor            Actor
	TenantScope      TenantScope
	PermissionIntent PermissionIntent
	UoW              UnitOfWork
}

func (e ExecutionContext) BaseContext() context.Context {
	if e.Context == nil {
		return context.Background()
	}
	return e.Context
}

func (e ExecutionContext) ValidatePermissioned() error {
	if e.Actor.Empty() {
		return NewError(ErrCodeInvalidExecutionContext, "actor is required")
	}
	if e.TenantScope.Empty() {
		return NewError(ErrCodeInvalidExecutionContext, "tenant scope is required")
	}
	if e.PermissionIntent.Empty() {
		return NewError(ErrCodeInvalidExecutionContext, "permission intent is required")
	}
	return nil
}
