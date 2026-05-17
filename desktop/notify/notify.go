// Package notify wraps desktop-level notification mechanics (system tray,
// OS-native toast) so the rest of the codebase can call a single Notifier
// abstraction. On macOS / Linux / Windows beeep is used.
package notify

import "github.com/gen2brain/beeep"

// Notifier delivers a desktop notification. Implementations must be safe
// to call from any goroutine.
type Notifier interface {
	Notify(title, body string) error
	Alert(title, body string) error
}

// Noop discards all notifications. Useful in tests or when running headless.
type Noop struct{}

func (Noop) Notify(string, string) error { return nil }
func (Noop) Alert(string, string) error  { return nil }

// Desktop is the default Notifier, backed by beeep.
type Desktop struct {
	AppIcon string // optional path to icon file
}

// NewDesktop returns a Desktop Notifier with no icon set.
func NewDesktop() *Desktop { return &Desktop{} }

func (d *Desktop) Notify(title, body string) error {
	return beeep.Notify(title, body, d.AppIcon)
}

func (d *Desktop) Alert(title, body string) error {
	return beeep.Alert(title, body, d.AppIcon)
}
