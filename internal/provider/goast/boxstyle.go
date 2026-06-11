// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package goast

// BoxStyle defines a character set for rendering delineator comments.
type BoxStyle struct {
	Name        string
	Horizontal  rune
	Vertical    rune // zero for line-only and ASCII styles
	TopLeft     rune // zero for line-only and ASCII styles
	TopRight    rune
	BottomLeft  rune
	BottomRight rune
}

// HasCorners returns true if the style has corner characters for box rendering.
func (s BoxStyle) HasCorners() bool {
	return s.TopLeft != 0
}

var boxStyles = map[string]BoxStyle{
	// Box styles (have corners — usable for lines, banners, and boxes).
	"light":   {Name: "light", Horizontal: '─', Vertical: '│', TopLeft: '┌', TopRight: '┐', BottomLeft: '└', BottomRight: '┘'},
	"heavy":   {Name: "heavy", Horizontal: '━', Vertical: '┃', TopLeft: '┏', TopRight: '┓', BottomLeft: '┗', BottomRight: '┛'},
	"double":  {Name: "double", Horizontal: '═', Vertical: '║', TopLeft: '╔', TopRight: '╗', BottomLeft: '╚', BottomRight: '╝'},
	"rounded": {Name: "rounded", Horizontal: '─', Vertical: '│', TopLeft: '╭', TopRight: '╮', BottomLeft: '╰', BottomRight: '╯'},

	// Line-only styles (no corners — lines and banners only).
	"light-triple-dash": {Name: "light-triple-dash", Horizontal: '┄', Vertical: '┆'},
	"heavy-triple-dash": {Name: "heavy-triple-dash", Horizontal: '┅', Vertical: '┇'},
	"light-quad-dash":   {Name: "light-quad-dash", Horizontal: '┈', Vertical: '┊'},
	"heavy-quad-dash":   {Name: "heavy-quad-dash", Horizontal: '┉', Vertical: '┋'},
	"light-double-dash": {Name: "light-double-dash", Horizontal: '╌', Vertical: '╎'},
	"heavy-double-dash": {Name: "heavy-double-dash", Horizontal: '╍', Vertical: '╏'},

	// ASCII styles (single repeated character).
	"ascii":   {Name: "ascii", Horizontal: '-'},
	"ascii-=": {Name: "ascii-=", Horizontal: '='},
	"ascii-*": {Name: "ascii-*", Horizontal: '*'},
	"ascii-#": {Name: "ascii-#", Horizontal: '#'},
	"ascii-~": {Name: "ascii-~", Horizontal: '~'},
	"ascii-+": {Name: "ascii-+", Horizontal: '+'},
	"ascii-_": {Name: "ascii-_", Horizontal: '_'},
}

// LookupBoxStyle returns the named style, or nil if not found.
func LookupBoxStyle(name string) *BoxStyle {
	if s, ok := boxStyles[name]; ok {
		return &s
	}
	return nil
}
