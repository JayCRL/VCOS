package lockmgr

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAcquireReleaseHappyPath(t *testing.T) {
	m := New(nil)
	defer m.Close()

	lease, err := m.Acquire("k1", "owner-a", time.Second)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if lease.Depth != 1 {
		t.Fatalf("depth=%d want 1", lease.Depth)
	}
	if owner, _ := m.Holder("k1"); owner != "owner-a" {
		t.Fatalf("Holder=%q", owner)
	}

	if err := m.Release("k1", "owner-a"); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if owner, _ := m.Holder("k1"); owner != "" {
		t.Fatalf("expected free, got owner=%q", owner)
	}
}

func TestAcquireConflict(t *testing.T) {
	m := New(nil)
	defer m.Close()

	if _, err := m.Acquire("k", "a", time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Acquire("k", "b", time.Second); err != ErrConflict {
		t.Fatalf("got err=%v want ErrConflict", err)
	}
}

func TestReentrant(t *testing.T) {
	m := New(nil)
	defer m.Close()

	if _, err := m.Acquire("k", "a", time.Second); err != nil {
		t.Fatal(err)
	}
	lease2, err := m.Acquire("k", "a", time.Second)
	if err != nil {
		t.Fatalf("reentrant: %v", err)
	}
	if lease2.Depth != 2 {
		t.Fatalf("depth=%d want 2", lease2.Depth)
	}
	if err := m.Release("k", "a"); err != nil {
		t.Fatal(err)
	}
	if owner, _ := m.Holder("k"); owner != "a" {
		t.Fatalf("expected still held after one release, got %q", owner)
	}
	if err := m.Release("k", "a"); err != nil {
		t.Fatal(err)
	}
	if owner, _ := m.Holder("k"); owner != "" {
		t.Fatalf("expected free, got %q", owner)
	}
}

func TestExpire(t *testing.T) {
	m := New(nil)
	defer m.Close()

	if _, err := m.Acquire("k", "a", 30*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	select {
	case lease := <-m.Expired():
		if lease.Key != "k" || lease.Owner != "a" {
			t.Fatalf("unexpected expired lease: %+v", lease)
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive expiration")
	}
	if owner, _ := m.Holder("k"); owner != "" {
		t.Fatalf("Holder after expire = %q", owner)
	}
}

func TestForceRelease(t *testing.T) {
	m := New(nil)
	defer m.Close()

	if _, err := m.Acquire("k", "a", time.Hour); err != nil {
		t.Fatal(err)
	}
	m.ForceRelease("k")
	if owner, _ := m.Holder("k"); owner != "" {
		t.Fatalf("Holder=%q after ForceRelease", owner)
	}
}

func TestConcurrentAcquireRelease(t *testing.T) {
	m := New(nil)
	defer m.Close()

	const owners = 8
	const rounds = 50
	var success atomic.Int64
	var conflict atomic.Int64

	wg := sync.WaitGroup{}
	wg.Add(owners)
	for i := 0; i < owners; i++ {
		owner := "o" + itoa(uint64(i))
		go func() {
			defer wg.Done()
			for j := 0; j < rounds; j++ {
				if _, err := m.Acquire("shared", owner, time.Second); err == nil {
					success.Add(1)
					_ = m.Release("shared", owner)
				} else {
					conflict.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	if success.Load()+conflict.Load() != owners*rounds {
		t.Fatalf("totals mismatch: success=%d conflict=%d", success.Load(), conflict.Load())
	}
	if m.Active() != 0 {
		t.Fatalf("Active=%d want 0", m.Active())
	}
}

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
