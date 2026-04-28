//go:build !debug

//nolint:all
package main

import (
	"context"
)

type debugConfig struct{}

//nolint:all
func hasDebugFlags() bool {
	return false
}

func registerDebugFlags() *debugConfig {
	return &debugConfig{}
}

func applyDebug(_ *debugConfig, _ *runOptions) func(context.Context) {
	return func(context.Context) {}
}
