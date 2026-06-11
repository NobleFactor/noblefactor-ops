// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package goast

import "testing"

func TestIsDelineatorLine_PureRepeated(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		// ASCII fillers — all recognized characters.
		{"equals", "===", true},
		{"dashes", "---", true},
		{"tildes", "~~~", true},
		{"stars", "***", true},
		{"hashes", "###", true},
		{"plus", "+++", true},
		{"carets", "^^^", true},
		{"slashes", "///", true},
		{"at", "@@@", true},
		{"percent", "%%%", true},
		{"underscore", "___", true},

		// Long lines.
		{"long equals", "=============================================================================", true},
		{"long dashes", "-----------------------------------------------------------------------------", true},

		// Unicode box-drawing.
		{"light horizontal", "───────────", true},
		{"heavy horizontal", "━━━━━━━━━━━", true},
		{"double horizontal", "═══════════", true},
		{"light triple dash", "┄┄┄┄┄┄┄┄┄┄┄", true},
		{"heavy triple dash", "┅┅┅┅┅┅┅┅┅┅┅", true},
		{"light quad dash", "┈┈┈┈┈┈┈┈┈┈┈", true},
		{"heavy quad dash", "┉┉┉┉┉┉┉┉┉┉┉", true},
		{"light double dash", "╌╌╌╌╌╌╌╌╌╌╌", true},
		{"heavy double dash", "╍╍╍╍╍╍╍╍╍╍╍", true},

		// Too short.
		{"two chars", "==", false},
		{"one char", "=", false},
		{"empty", "", false},

		// Not a filler character.
		{"letters", "aaa", false},
		{"digits", "111", false},
		{"spaces", "   ", false},
		{"dots only", "...", false},

		// Mixed characters (not pure repeated).
		{"mixed", "=-=", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDelineatorLine(tt.line)
			if got != tt.want {
				t.Errorf("isDelineatorLine(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestIsDelineatorLine_CenteredBanner(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{"equals banner", "=== Section Name ===", true},
		{"dashes banner", "--- Helpers ---", true},
		{"stars banner", "*** Public API ***", true},
		{"hashes banner", "### Constants ###", true},
		{"long banner", "========================= Section Name ======================================", true},
		{"unicode banner", "━━━ Section Name ━━━", true},
		{"double banner", "═══ Section Name ═══", true},

		// Asymmetric filler counts (still valid).
		{"asymmetric", "=== Section Name ==========", true},

		// Too few leading fillers.
		{"short lead", "== Section Name ===", false},

		// Too few trailing fillers.
		{"short trail", "=== Section Name ==", false},

		// No text between fillers (that's a pure repeated line, not a banner).
		{"no text", "=== ===", true}, // spaces count as "text" between fillers

		// Different filler on each side (not a banner — filler must be the same).
		{"mixed fillers", "=== Section Name ---", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDelineatorLine(tt.line)
			if got != tt.want {
				t.Errorf("isDelineatorLine(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestIsDelineatorBlock_MultiLine(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{
			"box comment",
			"=============================================================================\nSection Name\n=============================================================================",
			true,
		},
		{
			"single delineator line in multi-line",
			"Some text\n---\nMore text",
			true,
		},
		{
			"banner in multi-line",
			"some comment\n=== Section ===\nmore text",
			true,
		},
		{
			"no delineator lines",
			"Just some\nplain text\nnothing special",
			false,
		},
		{
			"single line delineator",
			"===",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDelineatorBlock(tt.raw)
			if got != tt.want {
				t.Errorf("isDelineatorBlock(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestIsDelineatorRune(t *testing.T) {
	// Spot-check ASCII.
	for _, r := range []rune{'=', '-', '~', '*', '#', '+', '^', '/', '@', '%', '_'} {
		if !isDelineatorRune(r) {
			t.Errorf("isDelineatorRune(%q) = false, want true", r)
		}
	}

	// Spot-check non-fillers.
	for _, r := range []rune{'a', '1', '.', ' ', '!'} {
		if isDelineatorRune(r) {
			t.Errorf("isDelineatorRune(%q) = true, want false", r)
		}
	}

	// Spot-check Unicode box-drawing.
	for _, r := range []rune{'─', '━', '═', '┄', '┅', '┈', '┉', '╌', '╍'} {
		if !isDelineatorRune(r) {
			t.Errorf("isDelineatorRune(%q) = false, want true", r)
		}
	}
}
