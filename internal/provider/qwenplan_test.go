package provider

import "testing"

// Alibaba Cloud's Token Plan (Jeremy on Discord): its own host, apart from
// DashScope's pay as you go, with chat completions and Anthropic messages.
func TestQwenTokenPlan(t *testing.T) {
	p, err := FromPreset("qwen-token-plan")
	if err != nil {
		t.Fatal(err)
	}
	if p.Chat != "https://token-plan.maas.qianwenaiapi.com/compatible-mode/v1" || p.Anthropic != "https://token-plan.maas.qianwenaiapi.com/apps/anthropic" || p.Responses != "" {
		t.Fatalf("endpoints: %q %q %q", p.Chat, p.Responses, p.Anthropic)
	}
	if got := p.planModels(nil); len(got) != len(Preset("qwen-token-plan").Models) || got[1].ID != "qwen3.8-max" {
		t.Fatalf("plan's: %+v", got)
	}
	im, _ := imported("Qwen", "sk-sp-x", endpoints{chat: "https://token-plan.maas.qianwenaiapi.com/compatible-mode/v1"}, nil)
	if im.Preset != "qwen-token-plan" {
		t.Fatalf("imported: %+v", im)
	}
}
