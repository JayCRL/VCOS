package eventbus

import "time"

// Sink is a function type compatible with kernel.EventSink (intentionally not
// importing kernel here to avoid an import cycle — eventbus is a leaf package
// and kernel can wrap raw funcs into its own EventSink type at the call site).
type Sink func(any)

// WrapSink returns a Sink that first delegates to the underlying sink and then
// publishes the same payload onto the bus, tagged with the given Source.
//
// If sessionID is empty, the topic helper will try to derive one from the
// payload via SessionIDOf.
func WrapSink(sink Sink, src Source, sessionID string, bus Bus) Sink {
	if bus == nil {
		if sink == nil {
			return func(any) {}
		}
		return sink
	}
	return func(ev any) {
		if sink != nil {
			sink(ev)
		}
		if ev == nil {
			return
		}
		sid := sessionID
		if sid == "" {
			sid = SessionIDOf(ev)
		}
		bus.Publish(Envelope{
			Source:    src,
			Topic:     TopicOf(ev),
			SessionID: sid,
			Timestamp: time.Now(),
			Payload:   ev,
		})
	}
}
