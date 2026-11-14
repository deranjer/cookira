package recipes

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestStructuredIngredientValidation(t *testing.T) {
	for _, e := range []Ingredient{
		{Name: "onion", Quantity: ptr(-1.0)}, {Name: "onion", Quantity: ptr(math.NaN())},
		{Name: "onion", Quantity: ptr(2.0), QuantityMax: ptr(1.0)},
		{Name: "salt", Unit: "tsp"}, {Text: "1 onion", Name: "onion"}, {},
	} {
		g := Group{Entries: []Ingredient{e}}
		if err := normalizeIngredients(&g); !errors.Is(err, ErrInvalid) {
			t.Errorf("accepted invalid entry %#v: %v", e, err)
		}
	}
	g := Group{Entries: []Ingredient{{Name: "onion", Quantity: ptr(2.0), QuantityMax: ptr(3.0), Preparation: "diced", Optional: true}, {Name: "salt", Notes: "to taste"}, {Text: "1 (14 oz) can tomatoes"}}}
	if err := normalizeIngredients(&g); err != nil {
		t.Fatal(err)
	}
	if g.Items[0] != "2–3 onion, diced (optional)" || g.Items[1] != "salt, to taste" || g.Items[2] != "1 (14 oz) can tomatoes" {
		t.Fatalf("unexpected display: %#v", g.Items)
	}
}

func TestIngredientCatalogLifecycle(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	r, err := svc.Create(ctx, Fields{Title: ptr(uniq("Structured")), Ingredients: &[]Group{{Name: "Sauce", Entries: []Ingredient{
		{Name: "scallions", Quantity: ptr(2.0), Preparation: "diced"},
		{IngredientID: "green-onion", Quantity: ptr(3.0), Preparation: "sliced"},
		{Name: "olive oil", Quantity: ptr(1.0), Unit: "tablespoons"},
	}}}})
	if err != nil {
		t.Fatal(err)
	}
	entries := r.Ingredients[0].Entries
	if entries[0].IngredientID != "green-onion" || entries[1].Name != "green onion" || entries[2].Unit != "tbsp" {
		t.Fatalf("not canonical: %#v", entries)
	}
	// An older client edits metadata and adds a text line. Existing structured
	// entries survive, and the added line remains an explicit fallback.
	oldItems := append(append([]string{}, r.Ingredients[0].Items...), "pepper to taste")
	updated, err := svc.Update(ctx, r.ID, Fields{Ingredients: &[]Group{{Name: "Sauce", Items: oldItems}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Ingredients[0].Entries) != 4 || updated.Ingredients[0].Entries[0].IngredientID != "green-onion" || updated.Ingredients[0].Entries[3].Text != "pepper to taste" {
		t.Fatalf("legacy edit lost structure: %#v", updated.Ingredients)
	}
	updated, err = svc.AddIngredient(ctx, r.ID, "Sauce", "1 lime")
	if err != nil || len(updated.Ingredients[0].Entries) != 5 {
		t.Fatalf("append failed: %v %#v", err, updated.Ingredients)
	}
	restored, err := svc.Restore(ctx, r.ID, 1)
	if err != nil || len(restored.Ingredients[0].Entries) != 3 {
		t.Fatalf("restore failed: %v", err)
	}

	name := uniq("Test ingredient")
	alias := uniq("Test alias")
	c, err := svc.SaveCatalog(ctx, CatalogIngredient{Name: name, Category: "Produce", Aliases: []string{alias}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.SaveCatalog(ctx, CatalogIngredient{Name: uniq("Other"), Aliases: []string{alias}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("alias collision accepted: %v", err)
	}
	byAlias, err := svc.Create(ctx, Fields{Title: ptr(uniq("Alias recipe")), Ingredients: &[]Group{{Entries: []Ingredient{{Name: alias, Quantity: ptr(1.0)}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if byAlias.Ingredients[0].Entries[0].IngredientID != c.ID {
		t.Fatal("alias created a duplicate ingredient")
	}
	c.Category = "Pantry"
	c.Name = uniq("Renamed ingredient")
	if _, err = svc.SaveCatalog(ctx, c); err != nil {
		t.Fatal(err)
	}
	view, err := svc.Get(ctx, byAlias.ID)
	if err != nil {
		t.Fatal(err)
	}
	if e := view.Ingredients[0].Entries[0]; e.Name != c.Name || e.Category != "Pantry" {
		t.Fatalf("catalog edits not reflected: %#v", e)
	}
	legacyEdit, err := svc.Update(ctx, view.ID, Fields{Ingredients: &[]Group{{Items: view.Ingredients[0].Items}}})
	if err != nil {
		t.Fatal(err)
	}
	if legacyEdit.Ingredients[0].Entries[0].IngredientID != c.ID {
		t.Fatal("legacy edit after catalog rename lost identity")
	}
	if _, err = svc.Create(ctx, Fields{Title: ptr(uniq("Bad reference")), Ingredients: &[]Group{{Entries: []Ingredient{{IngredientID: "missing"}}}}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown id accepted: %v", err)
	}
}
