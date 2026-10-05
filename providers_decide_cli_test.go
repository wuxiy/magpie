package main

import (
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/provider"
)

// Bailian's decision model is added with its workspace or plan, and a
// System One API of another vendor with decide= (#647).
func TestProviderDecidePairs(t *testing.T) {
	p, err := provider.FromPreset("bailian-decision")
	if err != nil {
		t.Fatal(err)
	}
	if err := applyPairs(&p, []string{"workspace=ws-1"}); err != nil {
		t.Fatal(err)
	}
	if p.Decide != "https://ws-1.cn-beijing.maas.aliyuncs.com/compatible-mode/v1" {
		t.Fatalf("workspace: %s", p.Decide)
	}
	p, _ = provider.FromPreset("bailian-decision")
	if err := applyPairs(&p, []string{"workspace=ws-2", "region=ap-southeast-1"}); err != nil {
		t.Fatal(err)
	}
	if p.Decide != "https://ws-2.ap-southeast-1.maas.aliyuncs.com/compatible-mode/v1" {
		t.Fatalf("Singapore: %s", p.Decide)
	}
	p, _ = provider.FromPreset("bailian-decision")
	if err := applyPairs(&p, []string{"plan=token-plan"}); err != nil {
		t.Fatal(err)
	}
	if p.Decide != "https://token-plan.cn-beijing.maas.aliyuncs.com/compatible-mode/v1" || !strings.Contains(p.KeysURL, "token-plan") {
		t.Fatalf("Token Plan: %s %s", p.Decide, p.KeysURL)
	}
	if err := applyPairs(&p, []string{"workspace=ws-3"}); err == nil {
		t.Fatal("a workspace taken for the Token Plan")
	}
	if err := applyPairs(&p, []string{"region=mars"}); err == nil || !strings.Contains(err.Error(), "token-plan") {
		t.Fatalf("unknown region: %v", err)
	}
	c := provider.Provider{Name: "My Decider"}
	if err := applyPairs(&c, []string{"decide=https://decide.example.com/v1", "models=my-decision-model", "key=sk-1"}); err != nil {
		t.Fatal(err)
	}
	if !c.DecideOnly() || c.Jev() != "my-decision-model" || !c.DecidesModel("my-decision-model") {
		t.Fatalf("custom: %+v asked as %s", c, c.Jev())
	}
}
