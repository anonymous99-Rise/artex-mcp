package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestEveryToolIsWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, tl := range All() {
		if tl.Name == "" || tl.Description == "" || tl.Handler == nil {
			t.Fatalf("incomplete tool: %+v", tl)
		}
		if seen[tl.Name] {
			t.Fatalf("duplicate tool name: %s", tl.Name)
		}
		seen[tl.Name] = true
		if tl.InputSchema["type"] != "object" {
			t.Fatalf("%s schema is not an object", tl.Name)
		}
	}
	if len(All()) < 25 {
		t.Fatalf("only %d tools registered; expected the full surface", len(All()))
	}
}

func TestDefsAnnotations(t *testing.T) {
	for _, d := range Defs() {
		ann, ok := d["annotations"].(map[string]any)
		if !ok {
			t.Fatalf("%v missing annotations", d["name"])
		}
		if _, ok := ann["readOnlyHint"]; !ok {
			t.Fatalf("%v missing readOnlyHint", d["name"])
		}
	}
}

func TestDispatchUnknownTool(t *testing.T) {
	_, err := Dispatch(context.Background(), nil, "nope", nil)
	if !errors.Is(err, ErrUnknownTool) {
		t.Fatalf("err = %v, want ErrUnknownTool", err)
	}
}

func TestArgAccessors(t *testing.T) {
	a := Args{
		"s":    "  hello  ",
		"i":    json.Number("42"),
		"f":    float64(7),
		"b":    true,
		"bs":   "false",
		"ints": []any{json.Number("1"), float64(2), "3"},
		"strs": []any{" a ", json.Number("9")},
	}
	if got := a.Str("s"); got != "hello" {
		t.Fatalf("Str = %q", got)
	}
	if got := a.Int("i", 0); got != 42 {
		t.Fatalf("Int(Number) = %d", got)
	}
	if got := a.Int("f", 0); got != 7 {
		t.Fatalf("Int(float) = %d", got)
	}
	if got := a.Int("missing", 5); got != 5 {
		t.Fatalf("Int(default) = %d", got)
	}
	if !a.Bool("b", false) || a.Bool("bs", true) {
		t.Fatalf("Bool parsing wrong")
	}
	ids, err := a.Int64s("ints")
	if err != nil || len(ids) != 3 || ids[0] != 1 || ids[1] != 2 || ids[2] != 3 {
		t.Fatalf("Int64s = %v, %v", ids, err)
	}
	strs, err := a.Strings("strs")
	if err != nil || len(strs) != 2 || strs[0] != "a" || strs[1] != "9" {
		t.Fatalf("Strings = %v, %v", strs, err)
	}
	if _, err := a.NeedStr("missing"); err == nil {
		t.Fatal("NeedStr should fail on empty")
	}
}

func TestTruncateRuneSafe(t *testing.T) {
	s := strings.Repeat("中", 10)
	out := truncate(s, 7)
	if !strings.HasPrefix(out, "中中") {
		t.Fatalf("truncate broke runes: %q", out)
	}
	if !strings.HasSuffix(out, "…(输出已截断)") {
		t.Fatalf("missing truncation marker: %q", out)
	}
}

func TestRouteCatalogLoaded(t *testing.T) {
	if RouteCount() < 200 {
		t.Fatalf("route catalog has %d entries; generation likely failed", RouteCount())
	}
}
