package awake

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWant(t *testing.T) {
	now := time.Now()
	for _, c := range []struct {
		st   State
		want bool
	}{
		{State{}, false},
		{State{Busy: true}, true},
		{State{Last: now.Add(-time.Minute)}, true},
		{State{Last: now.Add(-Grace - time.Second)}, false},
	} {
		if got := want(c.st, now); got != c.want {
			t.Errorf("want(%+v) = %v, want %v", c.st, got, c.want)
		}
	}
}

// TestKeep holds sleep off while the agents work and the setting is on, once,
// and lets go when they are idle, when it is turned off and when Keep ends.
func TestKeep(t *testing.T) {
	oldHold, oldTick := hold, tick
	defer func() { hold, tick = oldHold, oldTick }()
	tick = time.Millisecond
	var held, takes atomic.Int32
	hold = func() (func(), error) {
		held.Add(1)
		takes.Add(1)
		return func() { held.Add(-1) }, nil
	}
	var mu sync.Mutex
	on, st := true, State{}
	set := func(o bool, s State) { mu.Lock(); on, st = o, s; mu.Unlock() }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		Keep(ctx, func() bool { mu.Lock(); defer mu.Unlock(); return on },
			func() State { mu.Lock(); defer mu.Unlock(); return st })
		close(done)
	}()
	wait := func(n int32, what string) {
		t.Helper()
		for end := time.Now().Add(2 * time.Second); held.Load() != n; {
			if time.Now().After(end) {
				t.Fatalf("%s: held %d, want %d", what, held.Load(), n)
			}
			time.Sleep(time.Millisecond)
		}
	}
	wait(0, "idle")
	set(true, State{Busy: true})
	wait(1, "busy")
	time.Sleep(20 * time.Millisecond)
	if takes.Load() != 1 {
		t.Fatalf("taken %d times while busy, want once", takes.Load())
	}
	set(true, State{Last: time.Now()})
	time.Sleep(20 * time.Millisecond)
	wait(1, "within grace")
	set(true, State{Last: time.Now().Add(-Grace - time.Minute)})
	wait(0, "past grace")
	set(false, State{Busy: true})
	time.Sleep(20 * time.Millisecond)
	wait(0, "off")
	set(true, State{Busy: true})
	wait(1, "on again")
	cancel()
	<-done
	wait(0, "ended")
}
