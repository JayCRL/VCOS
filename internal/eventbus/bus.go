package eventbus

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultPublishBuffer    = 4096
	defaultSubscriberBuffer = 256
	defaultDeliverTimeout   = 50 * time.Millisecond
)

// Bus is the pub/sub interface.
type Bus interface {
	Publish(env Envelope) int64
	Subscribe(name string, f Filter, h Handler) Subscription
	LatestCursor() int64
	Close() error
}

// Options tunes a Bus instance.
type Options struct {
	PublishBuffer    int
	SubscriberBuffer int
	DeliverTimeout   time.Duration
}

func (o Options) withDefaults() Options {
	if o.PublishBuffer <= 0 {
		o.PublishBuffer = defaultPublishBuffer
	}
	if o.SubscriberBuffer <= 0 {
		o.SubscriberBuffer = defaultSubscriberBuffer
	}
	if o.DeliverTimeout <= 0 {
		o.DeliverTimeout = defaultDeliverTimeout
	}
	return o
}

// New returns an in-memory Bus with default options.
func New() Bus { return NewWithOptions(Options{}) }

// NewWithOptions returns an in-memory Bus with the given options.
func NewWithOptions(opts Options) Bus {
	opts = opts.withDefaults()
	b := &memBus{
		opts:    opts,
		inbox:   make(chan Envelope, opts.PublishBuffer),
		subs:    make(map[string]*subscription),
		stopped: make(chan struct{}),
	}
	go b.dispatch()
	return b
}

type memBus struct {
	opts Options

	cursor atomic.Int64

	mu   sync.RWMutex
	subs map[string]*subscription
	seq  uint64

	inbox      chan Envelope
	stopped    chan struct{}
	closeOnce  sync.Once
	closeError error
}

func (b *memBus) Publish(env Envelope) int64 {
	if env.Timestamp.IsZero() {
		env.Timestamp = time.Now()
	}
	cursor := b.cursor.Add(1)
	env.Cursor = cursor
	select {
	case b.inbox <- env:
	case <-b.stopped:
	}
	return cursor
}

func (b *memBus) LatestCursor() int64 { return b.cursor.Load() }

func (b *memBus) Subscribe(name string, f Filter, h Handler) Subscription {
	if h == nil {
		return noopSub{}
	}
	b.mu.Lock()
	b.seq++
	id := name
	if id == "" {
		id = "sub"
	}
	id = id + "#" + itoa(b.seq)
	sub := &subscription{
		id:      id,
		filter:  f,
		handler: h,
		ch:      make(chan Envelope, b.opts.SubscriberBuffer),
		done:    make(chan struct{}),
		bus:     b,
	}
	b.subs[id] = sub
	b.mu.Unlock()
	go sub.run()
	return sub
}

func (b *memBus) detach(id string) {
	b.mu.Lock()
	sub, ok := b.subs[id]
	if ok {
		delete(b.subs, id)
	}
	b.mu.Unlock()
	if ok {
		sub.shutdown()
	}
}

func (b *memBus) Close() error {
	b.closeOnce.Do(func() {
		close(b.stopped)
		// drain inbox by closing it; dispatcher exits when stopped fires.
		b.mu.Lock()
		subs := make([]*subscription, 0, len(b.subs))
		for _, s := range b.subs {
			subs = append(subs, s)
		}
		b.subs = make(map[string]*subscription)
		b.mu.Unlock()
		for _, s := range subs {
			s.shutdown()
		}
	})
	return b.closeError
}

func (b *memBus) dispatch() {
	for {
		select {
		case <-b.stopped:
			return
		case env := <-b.inbox:
			b.mu.RLock()
			subs := make([]*subscription, 0, len(b.subs))
			for _, s := range b.subs {
				subs = append(subs, s)
			}
			b.mu.RUnlock()
			for _, s := range subs {
				if !s.filter.Match(env) {
					continue
				}
				s.deliver(env, b.opts.DeliverTimeout)
			}
		}
	}
}

type subscription struct {
	id      string
	filter  Filter
	handler Handler
	ch      chan Envelope
	done    chan struct{}
	bus     *memBus

	dropped     atomic.Uint64
	closeOnce   sync.Once
	stopHandler chan struct{}
}

func (s *subscription) ID() string { return s.id }

func (s *subscription) Close() {
	if s.bus != nil {
		s.bus.detach(s.id)
		return
	}
	s.shutdown()
}

func (s *subscription) shutdown() {
	s.closeOnce.Do(func() {
		close(s.ch)
		<-s.done
	})
}

// Dropped returns the number of envelopes shed because the subscriber was slow.
func (s *subscription) Dropped() uint64 { return s.dropped.Load() }

func (s *subscription) deliver(env Envelope, timeout time.Duration) {
	select {
	case s.ch <- env:
		return
	default:
	}
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case s.ch <- env:
	case <-t.C:
		s.dropped.Add(1)
	}
}

func (s *subscription) run() {
	defer close(s.done)
	for env := range s.ch {
		func() {
			defer func() { _ = recover() }()
			s.handler(env)
		}()
	}
}

type noopSub struct{}

func (noopSub) ID() string { return "" }
func (noopSub) Close()     {}

var _ = errors.New // reserved for future error returns

func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
