package mg

import "testing"

func TestGetExclusivity_Unrestricted_Apple(t *testing.T) {
	// Apple shows "iOS/Web App" on wiki but Discord IS the web app
	// So Apple should have no exclusivity restrictions
	ex := GetExclusivity("Apple")
	if ex != nil {
		t.Error("expected no exclusivity data for Apple (available everywhere)")
	}
	if !IsAvailableInGuild("Apple", "123456789") {
		t.Error("Apple should be available in any guild")
	}
}

func TestGetExclusivity_ServerExclusive(t *testing.T) {
	ex := GetExclusivity("Banana")
	if ex == nil {
		t.Fatal("expected exclusivity data for Banana")
	}
	if ex.NotOnDiscord {
		t.Error("expected Banana to be available on Discord")
	}
	if !ex.IsAvailableOnDiscord() {
		t.Error("expected Banana to be available on Discord")
	}
	// Banana requires even server ID (ends in 0,2,4,6,8)
	if !IsAvailableInGuild("Banana", "808935495543160852") {
		t.Error("expected Banana to be available in guild ending with 2")
	}
	if IsAvailableInGuild("Banana", "123456789") {
		t.Error("expected Banana to NOT be available in guild ending with 9")
	}
}

func TestGetExclusivity_Unrestricted(t *testing.T) {
	ex := GetExclusivity("Carrot")
	if ex != nil {
		t.Error("expected no exclusivity data for Carrot (unrestricted)")
	}
}

func TestIsAvailableInGuild(t *testing.T) {
	// Apple - available everywhere (no restrictions)
	if !IsAvailableInGuild("Apple", "123456780") {
		t.Error("Apple should be available everywhere")
	}

	// Banana - even server IDs (0,2,4,6,8)
	if !IsAvailableInGuild("Banana", "123456780") {
		t.Error("Banana should be available in guild ending with 0")
	}
	if IsAvailableInGuild("Banana", "123456781") {
		t.Error("Banana should NOT be available in guild ending with 1")
	}

	// Grape - server ID ending in 1
	if !IsAvailableInGuild("Grape", "123456781") {
		t.Error("Grape should be available in guild ending with 1")
	}
	if IsAvailableInGuild("Grape", "123456782") {
		t.Error("Grape should NOT be available in guild ending with 2")
	}

	// Lemon/Lychee - server ID ending in 2
	if !IsAvailableInGuild("Lemon", "123456782") {
		t.Error("Lemon should be available in guild ending with 2")
	}
	if !IsAvailableInGuild("Lychee", "123456782") {
		t.Error("Lychee should be available in guild ending with 2")
	}

	// Carrot - unrestricted
	if !IsAvailableInGuild("Carrot", "123456789") {
		t.Error("Carrot should be available everywhere")
	}
}

func TestFormatExclusivityBadge(t *testing.T) {
	tests := []struct {
		itemID   string
		expected string
	}{
		{"Apple", ""}, // Apple is available everywhere
		{"Banana", "🔒 Server Exclusive"},
		{"Carrot", ""},
	}
	for _, tt := range tests {
		got := FormatExclusivityBadge(tt.itemID)
		if got != tt.expected {
			t.Errorf("FormatExclusivityBadge(%q) = %q, want %q", tt.itemID, got, tt.expected)
		}
	}
}

func TestFormatExclusivityBadgeShort(t *testing.T) {
	tests := []struct {
		itemID   string
		expected string
	}{
		{"Apple", ""},
		{"Banana", "🔒"},
		{"banana", "🔒"}, // case insensitive
		{"Carrot", ""},
	}
	for _, tt := range tests {
		got := FormatExclusivityBadgeShort(tt.itemID)
		if got != tt.expected {
			t.Errorf("FormatExclusivityBadgeShort(%q) = %q, want %q", tt.itemID, got, tt.expected)
		}
	}
}

func TestFormatExclusivityDetail(t *testing.T) {
	// Apple - no restrictions
	detail := FormatExclusivityDetail("Apple", "")
	if detail != "" {
		t.Errorf("expected no detail for Apple, got %q", detail)
	}

	// Server exclusive in wrong guild (Banana needs even ID, 9 is odd)
	detail = FormatExclusivityDetail("Banana", "123456789")
	if detail == "" {
		t.Error("expected warning for server exclusive in wrong guild")
	}

	// Server exclusive in correct guild (Banana needs even ID, 2 is even)
	detail = FormatExclusivityDetail("Banana", "123456782")
	if detail == "" {
		t.Error("expected info for server exclusive in correct guild")
	}

	// Unrestricted item
	detail = FormatExclusivityDetail("Carrot", "")
	if detail != "" {
		t.Errorf("expected no detail for unrestricted item, got %q", detail)
	}
}
