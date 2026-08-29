//go:build !ios

package main

import (
	"context"
)

// withIOSPlatform is a no-op outside iOS; the plain box.Context registry
// wiring in state.go already applies.
func withIOSPlatform(ctx context.Context) context.Context {
	return ctx
}

func isIOSBuild() bool { return false }
