// Package awake keeps the computer from going to sleep by itself while
// agents work through magpie's gateway (xiao_wang24004 on X: the computer
// went to sleep in the middle of an agent's task). With settings.KeepAwake
// on, the magpie serving the gateway holds the system's sleep off while a
// request is in flight, a Claude Code turn waits on its tool results, or a
// request ended less than Grace ago — an agent running a build or the tests
// between one request and the next — and lets go once it is idle. Only the
// system's sleep of its own is held off: the display still turns off, and
// closing a laptop's lid or choosing Sleep still sleeps.
//
// On a Mac it is `caffeinate -i`, on Windows SetThreadExecutionState with
// ES_SYSTEM_REQUIRED, on Linux `systemd-inhibit --what=idle:sleep`; where
// none is there it does nothing.
package awake

import (
	"context"
	"log"
	"time"
)

// Grace is how long after the last request ended sleep is still held off.
var Grace = 10 * time.Minute

// tick is how often Keep looks again.
var tick = 5 * time.Second

// hold takes the system's sleep off until the release it returns is called;
// a test stands in for it.
var hold = takeHold

// State is what the gateway has in flight, as Keep needs to know it.
type State struct {
	Busy bool      // a request in flight or a turn waiting on tools
	Last time.Time // when the last request ended
}

// want is whether sleep is held off at now for st.
func want(st State, now time.Time) bool {
	return st.Busy || (!st.Last.IsZero() && now.Sub(st.Last) < Grace)
}

// Keep holds sleep off while on says to and state says the agents are at
// work, until ctx ends.
func Keep(ctx context.Context, on func() bool, state func() State) {
	var release func()
	let := func() {
		if release != nil {
			release()
			release = nil
			log.Print("awake: the agents are idle; the computer may sleep again")
		}
	}
	defer let()
	failed := false
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		if on() && want(state(), time.Now()) {
			if release == nil && !failed {
				r, err := hold()
				if err != nil {
					// not there (a Linux without systemd): said once
					log.Print("awake: can't keep the computer awake: ", err)
					failed = true
				} else {
					release = r
					log.Print("awake: agents at work; keeping the computer from sleeping")
				}
			}
		} else {
			let()
			failed = false
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
