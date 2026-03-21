package isolation

import (
	"strings"
	"testing"
)

func TestWrapInput_SafeModeDisabled(t *testing.T) {
	user, sys := WrapInput("my data", "be helpful", true, false)
	if user != "my data" {
		t.Errorf("user = %q, want unchanged %q", user, "my data")
	}
	if sys != "be helpful" {
		t.Errorf("sys = %q, want unchanged %q", sys, "be helpful")
	}
}

func TestWrapInput_NotExternal(t *testing.T) {
	user, sys := WrapInput("direct prompt", "be helpful", false, true)
	if user != "direct prompt" {
		t.Errorf("user = %q, want unchanged %q", user, "direct prompt")
	}
	if sys != "be helpful" {
		t.Errorf("sys = %q, want unchanged %q", sys, "be helpful")
	}
}

func TestWrapInput_WrapsUserInput(t *testing.T) {
	user, _ := WrapInput("some data", "", true, true)
	if !strings.HasPrefix(user, "<user_data_") {
		t.Errorf("wrapped user should start with <user_data_, got: %q", user)
	}
	if !strings.Contains(user, "some data") {
		t.Errorf("wrapped user should contain original content, got: %q", user)
	}
	// Opening and closing tags should match.
	lines := strings.SplitN(user, "\n", 2)
	openTag := lines[0]                        // e.g. <user_data_a3f8b2>
	closeTag := "</" + openTag[1:]             // e.g. </user_data_a3f8b2>
	if !strings.HasSuffix(user, closeTag+"\n") && !strings.HasSuffix(user, closeTag) {
		t.Errorf("closing tag %q not found at end of %q", closeTag, user)
	}
}

func TestWrapInput_AddsIsolationNote(t *testing.T) {
	_, sys := WrapInput("data", "", true, true)
	if !strings.Contains(sys, "external data") {
		t.Errorf("system prompt should contain isolation note, got: %q", sys)
	}
	if !strings.Contains(sys, "user_data_") {
		t.Errorf("system prompt should reference the tag name, got: %q", sys)
	}
}

func TestWrapInput_PreservesExistingSystemPrompt(t *testing.T) {
	_, sys := WrapInput("data", "analyze carefully", true, true)
	if !strings.Contains(sys, "analyze carefully") {
		t.Errorf("original system prompt should be preserved, got: %q", sys)
	}
	if !strings.Contains(sys, "external data") {
		t.Errorf("isolation note should be appended, got: %q", sys)
	}
}

func TestWrapInput_PlaceholderExpansion(t *testing.T) {
	userPrompt := "data"
	sysPrompt := "analyze the <{{DATA_TAG}}> content"
	user, sys := WrapInput(userPrompt, sysPrompt, true, true)

	// {{DATA_TAG}} should be replaced with the actual tag name in the system prompt.
	if strings.Contains(sys, "{{DATA_TAG}}") {
		t.Errorf("{{DATA_TAG}} placeholder should be expanded in system prompt, got: %q", sys)
	}
	if !strings.Contains(sys, "<user_data_") {
		t.Errorf("system prompt should contain the resolved tag, got: %q", sys)
	}

	// The tag used in user content and system prompt should match.
	// Extract tag name from user content (first line is the opening tag).
	openingLine := strings.SplitN(user, "\n", 2)[0]
	tagName := strings.Trim(openingLine, "<>")
	if !strings.Contains(sys, tagName) {
		t.Errorf("system prompt tag %q not found in sys %q", tagName, sys)
	}
}

func TestWrapInput_PlaceholderNotExpandedWhenUnsafe(t *testing.T) {
	// When safe=false, {{DATA_TAG}} must NOT be expanded (to avoid information leakage
	// and to preserve the user's text verbatim).
	sysPrompt := "analyze {{DATA_TAG}}"
	_, sys := WrapInput("data", sysPrompt, true, false)
	if sys != sysPrompt {
		t.Errorf("safe=false: system prompt should be unchanged, got: %q", sys)
	}
}

func TestWrapInput_NonceIsUnique(t *testing.T) {
	// Run multiple invocations and check that tag names differ (probabilistically).
	seen := make(map[string]bool)
	for i := 0; i < 10; i++ {
		user, _ := WrapInput("data", "", true, true)
		openingLine := strings.SplitN(user, "\n", 2)[0]
		tagName := strings.Trim(openingLine, "<>")
		seen[tagName] = true
	}
	if len(seen) < 2 {
		t.Error("expected unique nonce tags across invocations, but all were identical")
	}
}

func TestWrapInput_TagsMatchBetweenUserAndSystem(t *testing.T) {
	user, sys := WrapInput("my data", "", true, true)

	openingLine := strings.SplitN(user, "\n", 2)[0]
	tagName := strings.Trim(openingLine, "<>")

	if !strings.Contains(sys, tagName) {
		t.Errorf("system prompt should reference the same tag %q as user content, sys: %q", tagName, sys)
	}
}

func TestGenerateNonce_Length(t *testing.T) {
	nonce := generateNonce()
	if len(nonce) != 6 {
		t.Errorf("generateNonce() length = %d, want 6", len(nonce))
	}
}

func TestGenerateNonce_HexCharsOnly(t *testing.T) {
	nonce := generateNonce()
	for _, c := range nonce {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Errorf("generateNonce() contains non-hex char %q in %q", c, nonce)
		}
	}
}
