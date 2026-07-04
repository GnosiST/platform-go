package main

import (
	"log"

	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/config"
	"platform-go/internal/platform/core"
	"platform-go/internal/platform/httpapi"
)

func main() {
	cfg := config.Load()
	registry := capability.NewRegistry()
	for _, manifest := range core.DefaultManifests() {
		if err := registry.Register(manifest); err != nil {
			log.Fatalf("register capability %q: %v", manifest.ID, err)
		}
	}
	enabled := make([]capability.ID, 0, len(cfg.Capabilities))
	for _, id := range cfg.Capabilities {
		enabled = append(enabled, capability.ID(id))
	}
	ordered, err := registry.ResolveEnabled(enabled)
	if err != nil {
		log.Fatalf("resolve capabilities: %v", err)
	}
	server := httpapi.New(httpapi.ServerOptions{Capabilities: ordered})
	if err := server.Run(cfg.HTTPAddr); err != nil {
		log.Fatalf("run platform api: %v", err)
	}
}
