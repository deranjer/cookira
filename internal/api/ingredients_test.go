package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestIngredientCatalogAndStructuredRecipeAPI(t *testing.T) {
	e := newAuthEnv(t)
	name := fmt.Sprintf("API ingredient %d", time.Now().UnixNano())
	alias := name + " alias"
	// A new ingredient has no id; exercise the actual request schema too.
	catalog := decode(t, e.post("/api/v1/ingredients", map[string]any{
		"name": name, "category": "Pantry", "aliases": []string{alias},
	}, e.withSession), http.StatusOK)
	if catalog["id"] == "" {
		t.Fatal("missing catalog id")
	}
	recipe := decode(t, e.post("/api/v1/recipes", map[string]any{
		"title": name,
		"ingredients": []any{map[string]any{"name": "Main", "items": []string{}, "entries": []any{
			map[string]any{"name": alias, "quantity": 2, "unit": "tablespoons", "preparation": "chopped"},
		}}},
	}, e.withSession), http.StatusOK)
	group := recipe["ingredients"].([]any)[0].(map[string]any)
	entry := group["entries"].([]any)[0].(map[string]any)
	if entry["ingredientId"] != catalog["id"] || entry["unit"] != "tbsp" {
		t.Fatalf("unresolved entry: %#v", entry)
	}
	decode(t, e.post("/api/v1/ingredients", map[string]any{"name": alias, "category": "Other", "aliases": []string{}}, e.withSession), http.StatusUnprocessableEntity)
	unauth := e.do(http.MethodGet, "/api/v1/ingredients", "", "", nil)
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("catalog exposed without login: %d", unauth.Code)
	}
}
