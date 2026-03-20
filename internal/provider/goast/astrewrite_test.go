// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package goast

import (
	"os"
	"strings"
	"testing"
)

func TestASTRewrite_Full(t *testing.T) {
	src := `// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

package foo

// Old summary for Alpha.
//
// Parameters:
//   - x: the input.
func Alpha(x int) {
	// Setup phase.
	y := x + 1

	// Compute result.
	_ = y * 2
}

// =============================================================================
// SECTION HEADER
// =============================================================================

// Old summary for Beta that is way too short
// and wraps awkwardly.
func Beta() string {
	return ""
}

// This is a floating prose comment that is way too short
// and could be reflowed.

// Gamma is already fine.
func Gamma() {}
`

	os.MkdirAll("/tmp/rewrite-real", 0755)
	os.WriteFile("/tmp/rewrite-real/synthetic-input.go", []byte(src), 0644)

	got, err := rewriteFileFromSource("test.go", src, 120)
	if err != nil {
		t.Fatalf("rewrite error: %v", err)
	}
	os.WriteFile("/tmp/rewrite-real/synthetic-output.go", []byte(got), 0644)

	t.Logf("Input:  /tmp/rewrite-real/synthetic-input.go")
	t.Logf("Output: /tmp/rewrite-real/synthetic-output.go")
	t.Logf("Output:\n%s", got)

	// Doc comments reflowed.
	if strings.Contains(got, "too short\n// and wraps") {
		t.Error("Beta not reflowed")
	}
	if !strings.Contains(got, "// Old summary for Beta that is way too short and wraps awkwardly.") {
		t.Error("Beta reflow missing")
	}

	// Floating prose reflowed.
	if !strings.Contains(got, "// This is a floating prose comment that is way too short and could be reflowed.") {
		t.Error("floating prose not reflowed")
	}

	// Preserved.
	if !strings.Contains(got, "// SPDX-License-Identifier: MIT") {
		t.Error("copyright missing")
	}
	if !strings.Contains(got, "// =====") {
		t.Error("delineator missing")
	}
	if !strings.Contains(got, "// Gamma is already fine.") {
		t.Error("Gamma comment missing")
	}

	// Body comments preserved.
	if !strings.Contains(got, "// Setup phase.") {
		t.Error("body comment 'Setup phase' missing")
	}
	if !strings.Contains(got, "// Compute result.") {
		t.Error("body comment 'Compute result' missing")
	}

	// Code intact.
	if !strings.Contains(got, "func Alpha(x int) {") {
		t.Error("Alpha corrupted")
	}
	if !strings.Contains(got, "y := x + 1") {
		t.Error("Alpha body corrupted")
	}
	if !strings.Contains(got, `return ""`) {
		t.Error("Beta body corrupted")
	}

	// Gap between doc comment and func keyword.
	if strings.Contains(got, ".func ") {
		t.Error("doc comment and func keyword merged (gap handling broken)")
	}
}

func TestASTRewrite_RealFiles(t *testing.T) {
	os.MkdirAll("/tmp/rewrite-real", 0755)

	files := []struct{ in, out string }{
		{"doctaxonomy/normalize.go", "/tmp/rewrite-real/normalize-output.go"},
		{"provider.go", "/tmp/rewrite-real/provider-output.go"},
		{"doctaxonomy/schema.go", "/tmp/rewrite-real/schema-output.go"},
	}

	for _, f := range files {
		src, _ := os.ReadFile(f.in)
		os.WriteFile(strings.Replace(f.out, "-output", "-input", 1), src, 0644)

		got, err := rewriteFileFromSource(f.in, string(src), 120)
		if err != nil {
			t.Fatalf("rewrite %s: %v", f.in, err)
		}
		os.WriteFile(f.out, []byte(got), 0644)
		t.Logf("Wrote %s", f.out)
	}

	// Normalize: all receivers survive.
	got, _ := os.ReadFile("/tmp/rewrite-real/normalize-output.go")
	output := string(got)
	for _, sig := range []string{
		"func (p *Paragraph) Normalize()",
		"func (d *Directive) Normalize()",
		"func (s *ParamSection) Normalize()",
		"func (s *ReturnSection) Normalize()",
		"func (c *CodeBlock) Normalize()",
		"func (h *Heading) Normalize()",
		"func (d *FuncDoc) Normalize()",
		"func (d *TypeDoc) Normalize()",
		"func (c *CopyrightDoc) Normalize()",
	} {
		if !strings.Contains(output, sig) {
			t.Errorf("normalize.go missing: %s", sig)
		}
	}
	if n := strings.Count(output, "func (c *CopyrightDoc) Normalize()"); n != 1 {
		t.Errorf("CopyrightDoc.Normalize appears %d times", n)
	}

	// Normalize: body comments survive.
	if !strings.Contains(output, "// Sort schema elements by Order.") {
		t.Error("normalize.go missing body comment: Sort schema elements")
	}
	if !strings.Contains(output, "// Classify parsed elements.") {
		t.Error("normalize.go missing body comment: Classify parsed elements")
	}

	// Provider: path::name preserved (no colon-splitting).
	got, _ = os.ReadFile("/tmp/rewrite-real/provider-output.go")
	if strings.Contains(string(got), ":: ") {
		t.Error("provider.go: path::name was colon-split")
	}
}
