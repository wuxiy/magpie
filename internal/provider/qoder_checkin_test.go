package provider

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeQoderCampaigns is Qoder's campaigns as one account sees them, behind
// the plugin's fetch, answering as wallechfox/qoder-checkin reads them: the
// day's credits (CLAIM_BENEFIT) among other campaigns, and a claim whose
// answer is under data.
type fakeQoderCampaigns struct {
	mu      sync.Mutex
	status  string // the day's campaign's claimStatus; "" none listed
	claimAs string // what a claim answers; "" CLAIMED
	expired bool   // the plugin marks the sign-in expired
	asked   []string
	bad     []string
}

func (f *fakeQoderCampaigns) serve(t *testing.T, site string) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		b, _ := io.ReadAll(r.Body)
		f.asked = append(f.asked, r.Method+" "+strings.TrimPrefix(r.URL.Path, "/sash/api/v1/me/campaigns"))
		if f.expired {
			w.Header().Set("X-Magpie-Sign-In", "expired")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		now := time.Now().Unix()
		switch r.Method + " " + r.URL.Path {
		case "GET /sash/api/v1/me/campaigns":
			if len(b) != 0 {
				f.bad = append(f.bad, "a GET with a body")
			}
			list := `{"campaignId":"invite","actionType":"INVITE","claimStatus":"CLAIMABLE","benefit":{"amount":500}}`
			if f.status != "" {
				list += fmt.Sprintf(`,{"campaignId":"daily-1","actionType":"CLAIM_BENEFIT","claimStatus":%q,"startAt":%d,"endAt":%d,"benefit":{"amount":100}}`, f.status, (now-3600)*1000, (now+3600)*1000)
			}
			fmt.Fprintf(w, `{"campaigns":[%s]}`, list)
		case "POST /sash/api/v1/me/campaigns/daily-1/claim":
			if string(b) != "{}" {
				f.bad = append(f.bad, "claim body "+string(b))
			}
			as := f.claimAs
			if as == "" {
				as, f.status = "CLAIMED", "CLAIMED"
			}
			fmt.Fprintf(w, `{"data":{"status":%q,"benefit":{"amount":100}}}`, as)
		default:
			f.bad = append(f.bad, "asked "+r.Method+" "+r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	old := qoderCampaignURLs[site]
	qoderCampaignURLs[site] = srv.URL + "/sash/api/v1/me/campaigns"
	t.Cleanup(func() { qoderCampaignURLs[site] = old })
}

func (f *fakeQoderCampaigns) take() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.asked
	f.asked = nil
	return out
}

func qoderCheckinFixture(t *testing.T, now *time.Time, accts ...qoderCheckinAcct) checkiner {
	c := newQoderCheckiner(func() []qoderCheckinAcct { return accts })
	c.path = filepath.Join(t.TempDir(), "qoder-checkin.json")
	c.now = func() time.Time { return *now }
	return c
}

// Qoder's daily credits (ARNO on Discord): the campaigns are read, the
// day's claimed while claimable, once a Beijing day; one claimed already
// is a check-in done, none listed is no event, and a refusal or an expired
// sign-in is a failure with its reason. Other campaigns are never claimed.
func TestQoderCheckin(t *testing.T) {
	if !strings.HasPrefix(qoderCampaignURLs["qoder"], "https://openapi.qoder.sh/") || !strings.HasPrefix(qoderCampaignURLs[QoderCNID], "https://openapi.qoder.com.cn/") {
		t.Fatalf("campaigns at %v", qoderCampaignURLs)
	}
	f := &fakeQoderCampaigns{status: "CLAIMABLE"}
	f.serve(t, "qoder")
	now := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	c := qoderCheckinFixture(t, &now, qoderCheckinAcct{User: "arno", On: true, site: "qoder", uid: "u1", card: "qoder", via: http.DefaultClient.Do})

	rs := c.checkinNow(t.Context(), false)
	if len(rs) != 1 || rs[0].Outcome != CheckinClaimed || rs[0].Credit != 100 || rs[0].By != "qoder" || !rs[0].Asked {
		t.Fatalf("claimable: %+v", rs)
	}
	if got := strings.Join(f.take(), ", "); got != "GET , POST /daily-1/claim" {
		t.Fatalf("asked %s", got)
	}
	// the day's answer holds: nothing asked again
	if rs := c.checkinNow(t.Context(), false); rs[0].Outcome != CheckinClaimed || rs[0].Asked || len(f.take()) != 0 {
		t.Fatalf("asked again the same day: %+v", rs)
	}

	// the next day, claimed already (in Qoder): no claim
	now = now.Add(24 * time.Hour)
	if rs := c.checkinNow(t.Context(), false); rs[0].Outcome != CheckinDone || rs[0].Credit != 100 {
		t.Fatalf("in already: %+v", rs)
	}
	if got := strings.Join(f.take(), ", "); got != "GET " {
		t.Fatalf("in already, asked %s", got)
	}

	// no daily campaign listed
	now = now.Add(24 * time.Hour)
	f.status = ""
	if rs := c.checkinNow(t.Context(), false); rs[0].Outcome != CheckinInactive {
		t.Fatalf("none listed: %+v", rs)
	}

	// listed, but neither claimable nor claimed for today: tried again
	now = now.Add(24 * time.Hour)
	f.status = "UPCOMING"
	if rs := c.checkinNow(t.Context(), false); rs[0].Outcome != CheckinFailed || !strings.Contains(rs[0].Msg, "aren't offered yet") {
		t.Fatalf("not offered: %+v", rs)
	}

	// a claim that doesn't come back CLAIMED
	now = now.Add(24 * time.Hour)
	f.status, f.claimAs = "CLAIMABLE", "FAILED"
	if rs := c.checkinNow(t.Context(), false); rs[0].Outcome != CheckinFailed || !strings.Contains(rs[0].Msg, "FAILED") {
		t.Fatalf("refused: %+v", rs)
	}

	// an expired sign-in says so
	now = now.Add(24 * time.Hour)
	f.claimAs, f.expired = "", true
	if rs := c.checkinNow(t.Context(), false); rs[0].Outcome != CheckinFailed || !strings.Contains(rs[0].Msg, "sign in again") {
		t.Fatalf("expired: %+v", rs)
	}
	if len(f.bad) != 0 {
		t.Fatalf("sent wrong: %v", f.bad)
	}
}

// Qoder CN's account is checked in on its own site and kept apart from a
// Qoder account of the same uid; a plugin before 0.2.7, which serves chat
// completions only, says to update it.
func TestQoderCNCheckin(t *testing.T) {
	intl := &fakeQoderCampaigns{status: "CLAIMED"}
	intl.serve(t, "qoder")
	cn := &fakeQoderCampaigns{status: "CLAIMABLE"}
	cn.serve(t, QoderCNID)
	now := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	c := qoderCheckinFixture(t, &now,
		qoderCheckinAcct{User: "arno", On: true, site: QoderCNID, uid: "u1", card: QoderCNID, via: http.DefaultClient.Do},
		qoderCheckinAcct{User: "hu", On: true, site: "qoder", uid: "u1", card: "qoder", via: http.DefaultClient.Do},
	)
	got := map[string]string{}
	for _, r := range c.checkinNow(t.Context(), false) {
		got[r.User] = r.Outcome
	}
	if got["arno"] != CheckinClaimed || got["hu"] != CheckinDone {
		t.Fatalf("checked in: %v", got)
	}
	if a := strings.Join(cn.take(), ", "); a != "GET , POST /daily-1/claim" {
		t.Fatalf("Qoder CN was asked %s", a)
	}
	if a := strings.Join(intl.take(), ", "); a != "GET " {
		t.Fatalf("Qoder was asked %s", a)
	}

	old := qoderCheckinAcct{User: "arno", On: true, site: "qoder", uid: "u2", via: func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 404, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"only chat completions are served"}}`))}, nil
	}}
	if r := qoderCheckin(t.Context(), old, time.Now()); r.Outcome != CheckinFailed || !strings.Contains(r.Msg, "0.2.7") {
		t.Fatalf("old plugin: %+v", r)
	}
}

// A Qoder account's usage card is told it is checked in, and how.
func TestQoderCheckinOnTheCard(t *testing.T) {
	accts := []qoderCheckinAcct{{User: "arno", On: true, site: "qoder", uid: "u1", card: "qoder"}}
	st := map[string]WorkBuddyCheckin{"qoder|u1": {Day: CheckinDay(time.Now()), Outcome: CheckinClaimed, Credit: 100}}
	qs := []SubscriptionQuota{{Provider: "qoder", User: "arno"}}
	out := markCheckins(qs, qoderCards(accts), st, "qoder")
	if !out[0].Checkins || out[0].CheckinBy != "qoder" || out[0].Checkin == nil || out[0].Checkin.Credit != 100 {
		t.Fatalf("card: %+v", out[0])
	}
}
