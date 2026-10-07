//go:build wireinject
// +build wireinject

package main

import (
	"github.com/flashbacks/api-service/internal/infrastructure/di"
	"github.com/google/wire"
)

// InitializeApp builds the complete application dependency graph using Wire.
// The implementation is generated into wire_gen.go.
func InitializeApp() (*App, error) {
	wire.Build(
		di.ApplicationSet,
		// App struct itself — Wire will fill it via struct provider.
		wire.Struct(new(App), "*"),
	)
	return nil, nil
}
