package config

import "testing"

type mockModelAliasReader map[string]string

func (m mockModelAliasReader) ModelAliases() map[string]string { return m }

func TestResolveModelDirectDeepSeek(t *testing.T) {
	got, ok := ResolveModel(nil, "deepseek-v4-flash")
	if !ok || got != "deepseek-v4-flash" {
		t.Fatalf("expected deepseek-v4-flash, got ok=%v model=%q", ok, got)
	}
}

func TestResolveModelUnknown(t *testing.T) {
	_, ok := ResolveModel(nil, "totally-custom-model")
	if ok {
		t.Fatal("expected unknown model to fail resolve")
	}
}

func TestResolveModelRejectsLegacyModelName(t *testing.T) {
	_, ok := ResolveModel(nil, "deepseek-chat")
	if ok {
		t.Fatalf("expected legacy model deepseek-chat to be rejected")
	}
}

func TestResolveModelAcceptsVisionAlias(t *testing.T) {
	got, ok := ResolveModel(nil, "deepseek-vision-chat")
	if !ok || got != "deepseek-vision-chat" {
		t.Fatalf("expected deepseek-vision-chat -> deepseek-vision-chat, got ok=%v model=%q", ok, got)
	}
}

func TestResolveModelRejectsThirdPartyAlias(t *testing.T) {
	_, ok := ResolveModel(nil, "gpt-4.1")
	if ok {
		t.Fatalf("expected third-party alias gpt-4.1 to be rejected")
	}
}

func TestResolveModelRejectsStoreAlias(t *testing.T) {
	_, ok := ResolveModel(mockModelAliasReader{
		"my-model": "deepseek-v4-flash",
	}, "my-model")
	if ok {
		t.Fatalf("expected store alias to be rejected")
	}
}

func TestClaudeModelsResponsePaginationFields(t *testing.T) {
	resp := ClaudeModelsResponse()
	if _, ok := resp["first_id"]; !ok {
		t.Fatalf("expected first_id in response: %#v", resp)
	}
	if _, ok := resp["last_id"]; !ok {
		t.Fatalf("expected last_id in response: %#v", resp)
	}
	if _, ok := resp["has_more"]; !ok {
		t.Fatalf("expected has_more in response: %#v", resp)
	}
}
