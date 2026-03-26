package api

import (
	"testing"
)

func TestVersionNotDev(t *testing.T) {
	// This test ensures Version is set via ldflags in production builds.
	// If you see this fail, build with: go build -ldflags "-X github.com/pusk-platform/pusk/internal/api.Version=vX.Y.Z"
	// or use: make build
	if Version == "" {
		t.Error("Version must not be empty")
	}
}
