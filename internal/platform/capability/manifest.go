package capability

import "context"

type ID string

type Manifest struct {
	ID           ID
	Name         string
	Version      string
	Dependencies []ID
	Hooks        Hooks
}

type Hooks struct {
	Configure        Hook
	Migrate          Hook
	Seed             Hook
	RegisterServices Hook
	RegisterRoutes   Hook
	RegisterAdmin    Hook
	Start            Hook
}

type Hook func(context.Context, Runtime) error

type Runtime struct{}
