package session

import (
	"context"
	"os/exec"

	adbpkg "mobilevc/internal/adb"
)

// HostInfoProvider abstracts host-level diagnostics that differ by environment.
// Desktop CLI consumers can inject a no-op implementation to skip ADB checks.
type HostInfoProvider interface {
	LookPath(name string) (string, error)
	ADBAvailable() bool
	ADBPath() string
	EmulatorAvailable() bool
	EmulatorPath() string
	ADBMessage() string
}

// DefaultHostInfoProvider is the production implementation using real exec.LookPath and adb.
var DefaultHostInfoProvider HostInfoProvider = &defaultHostInfoProvider{}

type defaultHostInfoProvider struct{}

func (p *defaultHostInfoProvider) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func (p *defaultHostInfoProvider) ADBAvailable() bool {
	return adbpkg.DetectStatus(context.Background()).ADBAvailable
}

func (p *defaultHostInfoProvider) ADBPath() string {
	path := adbpkg.DetectStatus(context.Background()).ADBPath
	if path == "" {
		return "not found"
	}
	return path
}

func (p *defaultHostInfoProvider) EmulatorAvailable() bool {
	return adbpkg.DetectStatus(context.Background()).EmulatorAvailable
}

func (p *defaultHostInfoProvider) EmulatorPath() string {
	path := adbpkg.DetectStatus(context.Background()).EmulatorPath
	if path == "" {
		return "not found"
	}
	return path
}

func (p *defaultHostInfoProvider) ADBMessage() string {
	return adbpkg.DetectStatus(context.Background()).Message
}
