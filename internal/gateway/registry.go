package gateway

import "mobilevc/internal/kernel"

// Backward-compatible re-exports from kernel.

type runtimeSession = kernel.RuntimeSession
type runtimeSessionRegistry = kernel.RuntimeSessionRegistry

var defaultRuntimeSessionReleaseAfter = kernel.DefaultReleaseAfter
var defaultRuntimeSessionPendingLimit = kernel.DefaultPendingLimit
var defaultRuntimeSessionSinkBufferSize = kernel.DefaultSinkBufferSize

var newRuntimeSession = kernel.NewRuntimeSession
var newRuntimeSessionRegistry = kernel.NewRuntimeSessionRegistry
var eventCursorFromEvent = kernel.EventCursorFromEvent
