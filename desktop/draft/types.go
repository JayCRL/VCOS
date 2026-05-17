package draft

// Phase indicates the lifecycle of a streaming draft.
type Phase string

const (
	PhaseThinking Phase = "thinking"
	PhaseChunk    Phase = "chunk"
	PhaseDone     Phase = "done"
	PhaseError    Phase = "error"
)
