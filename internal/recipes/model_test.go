package recipes

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	tests := map[string]string{
		"Homemade Chicken Ramen":  "homemade-chicken-ramen",
		"  Crab   Ceviche!! ":     "crab-ceviche",
		"Mom's 3-Bean Chili (v2)": "mom-s-3-bean-chili-v2",
		"日本語":                     "recipe",
		"":                        "recipe",
		strings.Repeat("a", 200):  strings.Repeat("a", maxSlugLen),
		"Deviled Eggs.md":         "deviled-eggs-md",
	}
	for in, want := range tests {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestApplyNormalizes(t *testing.T) {
	r := Recipe{Status: StatusDraft}
	title := "  Ramen "
	tags := []string{" Soup", "soup", "", "Noodles"}
	groups := []Group{
		{Name: " Broth ", Items: []string{" 2 cups stock ", "", "soy"}},
		{Name: "Empty", Items: []string{"  "}},
	}
	steps := []Step{{Text: " Boil ", Substeps: []string{" a ", ""}}, {Text: "   "}}
	if err := apply(&r, Fields{Title: &title, Tags: &tags, Ingredients: &groups, Steps: &steps}); err != nil {
		t.Fatal(err)
	}
	if r.Title != "Ramen" {
		t.Errorf("title = %q", r.Title)
	}
	if want := []string{"soup", "noodles"}; !reflect.DeepEqual(r.Tags, want) {
		t.Errorf("tags = %v, want %v", r.Tags, want)
	}
	if want := []Group{{Name: "Broth", Items: []string{"2 cups stock", "soy"}}}; !reflect.DeepEqual(r.Ingredients, want) {
		t.Errorf("ingredients = %#v, want %#v", r.Ingredients, want)
	}
	if want := []Step{{Text: "Boil", Substeps: []string{"a"}}}; !reflect.DeepEqual(r.Steps, want) {
		t.Errorf("steps = %#v, want %#v", r.Steps, want)
	}
}

func TestApplyRejectsBadInput(t *testing.T) {
	str := func(s string) *string { return &s }
	num := func(n int) *int { return &n }
	tests := map[string]Fields{
		"empty title":    {Title: str("  ")},
		"bad status":     {Title: str("x"), Status: str("archived")},
		"negative time":  {Title: str("x"), PrepMinutes: num(-1)},
		"bad slug":       {Title: str("x"), Slug: str("Not A Slug")},
		"bad source url": {Title: str("x"), SourceURL: str("javascript:alert(1)")},
	}
	for name, f := range tests {
		r := Recipe{Status: StatusDraft}
		if err := apply(&r, f); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}

func TestPatchLeavesUnsuppliedFieldsAlone(t *testing.T) {
	r := Recipe{Title: "Soup", Status: StatusDraft, Servings: 4, Tags: []string{"soup"}}
	status := StatusPublished
	if err := apply(&r, Fields{Status: &status}); err != nil {
		t.Fatal(err)
	}
	if r.Title != "Soup" || r.Servings != 4 || len(r.Tags) != 1 || r.Status != StatusPublished {
		t.Errorf("patch changed unrelated fields: %#v", r)
	}
}

func TestFramingDefaultsAndValidation(t *testing.T) {
	// A zero framing (an old snapshot, or a brand-new Recipe) becomes centred, no zoom.
	r := Recipe{Title: "T", Status: StatusDraft}
	if err := apply(&r, Fields{}); err != nil {
		t.Fatal(err)
	}
	if r.ImageFraming != defaultFraming {
		t.Fatalf("framing = %+v, want %+v", r.ImageFraming, defaultFraming)
	}

	// Each part patches independently.
	if err := apply(&r, Fields{ImageFocusX: ptr(0.2), ImageZoom: ptr(2.5)}); err != nil {
		t.Fatal(err)
	}
	if r.ImageFocusX != 0.2 || r.ImageFocusY != 0.5 || r.ImageZoom != 2.5 {
		t.Fatalf("framing = %+v", r.ImageFraming)
	}

	for name, f := range map[string]Fields{
		"x low":    {ImageFocusX: ptr(-0.1)},
		"x high":   {ImageFocusX: ptr(1.01)},
		"y high":   {ImageFocusY: ptr(2.0)},
		"zoom low": {ImageZoom: ptr(0.5)},
		"zoom big": {ImageZoom: ptr(4.5)},
		"nan":      {ImageFocusX: ptr(math.NaN())},
	} {
		c := r
		if err := apply(&c, f); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: apply = %v, want ErrInvalid", name, err)
		}
	}
}
