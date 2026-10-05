package access

import (
	"testing"
	"time"
)

func TestLimitWindow(t *testing.T) {
	loc := time.FixedZone("X", 8*3600)
	at := func(s string) time.Time {
		v, err := time.ParseInLocation("2006-01-02 15:04", s, loc)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	for _, c := range []struct{ period, now, start, reset string }{
		{"day", "2026-10-03 23:59", "2026-10-03 00:00", "2026-10-04 00:00"},
		{"day", "2026-12-31 00:00", "2026-12-31 00:00", "2027-01-01 00:00"},
		{"week", "2026-10-03 12:00", "2026-09-28 00:00", "2026-10-05 00:00"}, // a Saturday: from Monday
		{"week", "2026-10-04 23:00", "2026-09-28 00:00", "2026-10-05 00:00"}, // Sunday is the week's last day
		{"week", "2026-10-05 00:00", "2026-10-05 00:00", "2026-10-12 00:00"},
		{"month", "2026-10-31 18:00", "2026-10-01 00:00", "2026-11-01 00:00"},
		{"month", "2026-12-15 08:00", "2026-12-01 00:00", "2027-01-01 00:00"},
	} {
		start, reset := Window(c.period, at(c.now))
		if !start.Equal(at(c.start)) || !reset.Equal(at(c.reset)) {
			t.Errorf("%s at %s: %s – %s", c.period, c.now, start, reset)
		}
	}
}

func TestLimitValid(t *testing.T) {
	if l, err := (&Limit{Period: "day"}).Valid(); l != nil || err != nil {
		t.Fatal("no cap is no limit", l, err)
	}
	if l, err := (*Limit)(nil).Valid(); l != nil || err != nil {
		t.Fatal(l, err)
	}
	if l, err := (&Limit{Tokens: 5}).Valid(); err != nil || l.Period != "day" {
		t.Fatal("a period is a day unless said", l, err)
	}
	for _, bad := range []Limit{{Period: "hour", Tokens: 1}, {Period: "day", Tokens: -1}, {Period: "day", Cost: -2}} {
		if _, err := bad.Valid(); err == nil {
			t.Error("accepted", bad)
		}
	}
}
