package eventbus

import (
	"reflect"

	"mobilevc/internal/protocol"
)

// TopicOf returns a stable topic string for an event payload, preferring the
// `Type` field on protocol envelopes and falling back to the Go type name.
func TopicOf(payload any) string {
	switch e := payload.(type) {
	case protocol.Event:
		return e.Type
	case protocol.LogEvent:
		return e.Type
	case protocol.ProgressEvent:
		return e.Type
	case protocol.ErrorEvent:
		return e.Type
	case protocol.ClientActionAckEvent:
		return e.Type
	case protocol.PromptRequestEvent:
		return e.Type
	case protocol.InteractionRequestEvent:
		return e.Type
	case protocol.SessionStateEvent:
		return e.Type
	case protocol.AgentStateEvent:
		return e.Type
	case protocol.AIStatusEvent:
		return e.Type
	case protocol.RuntimePhaseEvent:
		return e.Type
	case protocol.TaskSnapshotEvent:
		return e.Type
	case protocol.StepUpdateEvent:
		return e.Type
	case protocol.FileDiffEvent:
		return e.Type
	case protocol.FSListResultEvent:
		return e.Type
	case protocol.FSReadResultEvent:
		return e.Type
	case protocol.SessionCreatedEvent:
		return e.Type
	case protocol.SessionListResultEvent:
		return e.Type
	case protocol.SessionHistoryEvent:
		return e.Type
	case protocol.SessionDeltaEvent:
		return e.Type
	case protocol.SessionResumeResultEvent:
		return e.Type
	case protocol.SessionResumeNoticeEvent:
		return e.Type
	case protocol.ReviewStateEvent:
		return e.Type
	case protocol.SkillCatalogResultEvent:
		return e.Type
	case protocol.MemoryListResultEvent:
		return e.Type
	case protocol.CatalogAuthoringResultEvent:
		return e.Type
	case protocol.SessionContextResultEvent:
		return e.Type
	case protocol.PermissionRuleListResultEvent:
		return e.Type
	case protocol.PermissionAutoAppliedEvent:
		return e.Type
	case protocol.SkillSyncResultEvent:
		return e.Type
	case protocol.CatalogSyncStatusEvent:
		return e.Type
	case protocol.CatalogSyncResultEvent:
		return e.Type
	case protocol.RuntimeInfoResultEvent:
		return e.Type
	case protocol.RuntimeProcessListResultEvent:
		return e.Type
	case protocol.RuntimeProcessLogResultEvent:
		return e.Type
	case protocol.ADBDevicesResultEvent:
		return e.Type
	case protocol.ADBStreamStateEvent:
		return e.Type
	case protocol.ADBFrameEvent:
		return e.Type
	case protocol.ADBWebRTCAnswerEvent:
		return e.Type
	case protocol.ADBWebRTCStateEvent:
		return e.Type
	}
	if payload == nil {
		return ""
	}
	t := reflect.TypeOf(payload)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Name()
}

// SessionIDOf extracts a SessionID from a payload when one is exposed via the
// embedded protocol.Event struct. Returns "" otherwise.
func SessionIDOf(payload any) string {
	switch e := payload.(type) {
	case protocol.Event:
		return e.SessionID
	case protocol.LogEvent:
		return e.SessionID
	case protocol.ProgressEvent:
		return e.SessionID
	case protocol.ErrorEvent:
		return e.SessionID
	case protocol.ClientActionAckEvent:
		return e.SessionID
	case protocol.PromptRequestEvent:
		return e.SessionID
	case protocol.InteractionRequestEvent:
		return e.SessionID
	case protocol.SessionStateEvent:
		return e.SessionID
	case protocol.AgentStateEvent:
		return e.SessionID
	case protocol.AIStatusEvent:
		return e.SessionID
	case protocol.RuntimePhaseEvent:
		return e.SessionID
	case protocol.TaskSnapshotEvent:
		return e.SessionID
	case protocol.StepUpdateEvent:
		return e.SessionID
	case protocol.FileDiffEvent:
		return e.SessionID
	case protocol.FSListResultEvent:
		return e.SessionID
	case protocol.FSReadResultEvent:
		return e.SessionID
	case protocol.SessionCreatedEvent:
		return e.SessionID
	case protocol.SessionListResultEvent:
		return e.SessionID
	case protocol.SessionHistoryEvent:
		return e.SessionID
	case protocol.SessionDeltaEvent:
		return e.SessionID
	case protocol.SessionResumeResultEvent:
		return e.SessionID
	case protocol.SessionResumeNoticeEvent:
		return e.SessionID
	case protocol.ReviewStateEvent:
		return e.SessionID
	case protocol.SkillCatalogResultEvent:
		return e.SessionID
	case protocol.MemoryListResultEvent:
		return e.SessionID
	case protocol.CatalogAuthoringResultEvent:
		return e.SessionID
	case protocol.SessionContextResultEvent:
		return e.SessionID
	case protocol.PermissionRuleListResultEvent:
		return e.SessionID
	case protocol.PermissionAutoAppliedEvent:
		return e.SessionID
	case protocol.SkillSyncResultEvent:
		return e.SessionID
	case protocol.CatalogSyncStatusEvent:
		return e.SessionID
	case protocol.CatalogSyncResultEvent:
		return e.SessionID
	case protocol.RuntimeInfoResultEvent:
		return e.SessionID
	case protocol.RuntimeProcessListResultEvent:
		return e.SessionID
	case protocol.RuntimeProcessLogResultEvent:
		return e.SessionID
	}
	return ""
}
