package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yetone/magpie/internal/provider"
)

// A remote magpie's quotas (莫 on Discord), between a gateway and a
// magpie that has it as its provider: the cards it is shown are the
// gateway's as last read, and asking for them asks no vendor, however
// often; a card's refresh reads it there once, and again no sooner than
// remoteRefreshGap; a card the gateway has from a third magpie isn't read
// for it; another machine without the gateway's key gets nothing.
func TestRemoteMagpieQuotas(t *testing.T) {
	fresh(t)
	t.Setenv("MAGPIE_ADDR", "")
	provider.ForgetBalances()
	t.Cleanup(provider.ForgetBalances)
	var reads atomic.Int32
	vendor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		w.Write([]byte(`{"data":{"total_available":6000000}}`))
	}))
	defer vendor.Close()
	if err := provider.Save(provider.Provider{ID: "bal", Name: "Bal", Key: "sk-bal", Chat: vendor.URL + "/v1",
		BalanceURL: vendor.URL + "/b", BalancePath: "$data.total_available / 500000"}); err != nil {
		t.Fatal(err)
	}
	gw := httptest.NewServer(lanGuard(New().Handler()))
	defer gw.Close()
	if err := provider.Save(provider.Provider{ID: "office", Name: "Office", Preset: provider.RemoteMagpiePreset, Key: "sk-magpie-office", Chat: gw.URL}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// nothing read on the gateway yet: nothing is read for the remote
	cs := provider.RemoteCards(ctx)
	if len(cs) != 1 || cs[0].Provider != "office" || !strings.HasPrefix(cs[0].Error, "nothing read") {
		t.Fatalf("before any read: %+v", cs)
	}
	if n := reads.Load(); n != 0 {
		t.Fatalf("the remote's ask read the vendor %d times", n)
	}
	// its card's refresh has the gateway read every card
	provider.RefreshUsage(ctx, "office", "")
	if n := reads.Load(); n != 1 {
		t.Fatalf("refresh read the vendor %d times, want 1", n)
	}
	var bal *provider.SubscriptionQuota
	for _, q := range provider.RemoteCards(ctx) {
		if q.Provider == "office/bal" {
			bal = &q
		}
	}
	if bal == nil || bal.Balance != "$12.00" || bal.Name != "Bal · Office" || bal.Kind != "balance" {
		t.Fatalf("office/bal: %+v", bal)
	}
	// asked again and again, the gateway answers from what it has
	for range 5 {
		r, _ := http.NewRequest("GET", gw.URL+provider.RemoteCardsPath, nil)
		res, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
	}
	if n := reads.Load(); n != 1 {
		t.Fatalf("asking for the cards read the vendor: %d", n)
	}
	// the card's own refresh, pressed twice at once: read once
	provider.RefreshUsage(ctx, "office/bal", "")
	provider.RefreshUsage(ctx, "office/bal", "")
	if n := reads.Load(); n != 2 {
		t.Fatalf("two refreshes read the vendor %d times in all, want 2", n)
	}
	post := func(from, q, key string) int {
		r := httptest.NewRequest("POST", provider.RemoteRefreshPath+q, nil)
		r.RemoteAddr = from
		if key != "" {
			r.Header.Set("Authorization", "Bearer "+key)
		}
		w := httptest.NewRecorder()
		lanGuard(New().Handler()).ServeHTTP(w, r)
		return w.Code
	}
	if c := post("127.0.0.1:5000", "?provider=home/codex", ""); c != http.StatusBadRequest {
		t.Errorf("a third magpie's card: %d", c)
	}
	if c := post("127.0.0.1:5000", "?provider=office", ""); c != http.StatusBadRequest {
		t.Errorf("a remote magpie's own card: %d", c)
	}
	if c := post("192.168.1.9:5000", "?provider=bal", ""); c == http.StatusOK {
		t.Error("another machine without a key had a card read")
	}
	for _, path := range []string{provider.RemoteCardsPath} {
		r := httptest.NewRequest("GET", path, nil)
		r.RemoteAddr = "192.168.1.9:5000"
		w := httptest.NewRecorder()
		lanGuard(New().Handler()).ServeHTTP(w, r)
		if w.Code == http.StatusOK {
			t.Errorf("another machine without a key got %s", path)
		}
	}
	if n := reads.Load(); n != 2 {
		t.Errorf("refused requests read the vendor: %d", n)
	}
}
