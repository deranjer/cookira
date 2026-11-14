package recipes

import (
	"context"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/deranjer/cookira/internal/db"
	"github.com/jackc/pgx/v5"
)

// Ingredient is a recipe's usage of a shared ingredient. A text-only entry is
// an explicit fallback for legacy/imported lines; it is never guessed at.
type Ingredient struct {
	IngredientID string   `json:"ingredientId,omitempty"`
	Name         string   `json:"name,omitempty"`
	Quantity     *float64 `json:"quantity,omitempty"`
	QuantityMax  *float64 `json:"quantityMax,omitempty"`
	Unit         string   `json:"unit,omitempty"`
	Size         string   `json:"size,omitempty"`
	Preparation  string   `json:"preparation,omitempty"`
	Optional     bool     `json:"optional,omitempty"`
	Notes        string   `json:"notes,omitempty"`
	OriginalText string   `json:"originalText,omitempty"`
	Text         string   `json:"text,omitempty"`
	Category     string   `json:"category,omitempty"`
}

type CatalogIngredient struct {
	ID       string   `json:"id,omitempty"`
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Aliases  []string `json:"aliases"`
}

func ingredientText(e Ingredient) string {
	if e.Text != "" {
		return e.Text
	}
	parts := []string{}
	if e.Quantity != nil {
		amount := strconv.FormatFloat(*e.Quantity, 'f', -1, 64)
		if e.QuantityMax != nil {
			amount += "–" + strconv.FormatFloat(*e.QuantityMax, 'f', -1, 64)
		}
		parts = append(parts, amount)
	}
	name := e.Name
	if name == "" {
		name = e.IngredientID
	}
	for _, s := range []string{e.Unit, e.Size, name} {
		if s != "" {
			parts = append(parts, s)
		}
	}
	text := strings.Join(parts, " ")
	for _, s := range []string{e.Preparation, e.Notes} {
		if s != "" {
			text += ", " + s
		}
	}
	if e.Optional {
		text += " (optional)"
	}
	return text
}

func normalizeIngredients(g *Group) error {
	if len(g.Entries) > maxItems {
		return invalid("at most %d ingredients per group", maxItems)
	}
	g.Items = make([]string, 0, len(g.Entries))
	for i := range g.Entries {
		e := &g.Entries[i]
		fields := []*string{&e.IngredientID, &e.Name, &e.Unit, &e.Size, &e.Preparation, &e.Notes, &e.OriginalText, &e.Text, &e.Category}
		for _, f := range fields {
			*f = strings.TrimSpace(*f)
			if len(*f) > maxTextLen {
				return invalid("ingredient fields must be under %d characters", maxTextLen)
			}
		}
		if e.Text != "" {
			if e.Name != "" || e.IngredientID != "" || e.Quantity != nil || e.QuantityMax != nil || e.Unit != "" || e.Size != "" || e.Preparation != "" || e.Notes != "" || e.Optional {
				return invalid("a text-only ingredient cannot also contain structured fields")
			}
		} else {
			if e.Name == "" && e.IngredientID == "" {
				return invalid("ingredient name is required")
			}
			for _, v := range []*float64{e.Quantity, e.QuantityMax} {
				if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0) || *v <= 0 || *v > 1_000_000) {
					return invalid("ingredient quantity must be greater than 0 and at most 1000000")
				}
			}
			if e.QuantityMax != nil && (e.Quantity == nil || *e.QuantityMax < *e.Quantity) {
				return invalid("maximum quantity must be at least the quantity")
			}
			if e.Unit != "" && e.Quantity == nil {
				return invalid("a unit requires a quantity; use notes for 'to taste' or 'as needed'")
			}
			e.Unit = canonicalUnit(e.Unit)
		}
		g.Items = append(g.Items, ingredientText(*e))
	}
	return nil
}

func canonicalUnit(unit string) string {
	unit = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(unit), "."))
	for canonical, aliases := range map[string]string{
		"tsp": "tsp teaspoon teaspoons", "tbsp": "tbsp tablespoon tablespoons",
		"cup": "cup cups", "g": "g gram grams", "kg": "kg kilogram kilograms",
		"oz": "oz ounce ounces", "lb": "lb lbs pound pounds", "ml": "ml milliliter milliliters millilitre millilitres",
		"l": "l liter liters litre litres", "clove": "clove cloves", "can": "can cans", "bunch": "bunch bunches",
	} {
		for _, alias := range strings.Fields(aliases) {
			if unit == alias {
				return canonical
			}
		}
	}
	return unit
}

func (s *Service) Catalog(ctx context.Context) ([]CatalogIngredient, error) {
	rows, err := s.pool.Query(ctx, `SELECT c.id, c.name, c.category, COALESCE(array_agg(n.name ORDER BY n.name) FILTER (WHERE n.name <> lower(c.name)), '{}') FROM ingredient_catalog c LEFT JOIN ingredient_names n ON n.ingredient_id=c.id GROUP BY c.id ORDER BY c.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CatalogIngredient{}
	for rows.Next() {
		var c CatalogIngredient
		if err := rows.Scan(&c.ID, &c.Name, &c.Category, &c.Aliases); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Catalog edits apply to current recipe views; revisions retain their snapshot.
func (s *Service) hydrateIngredients(ctx context.Context, r Recipe) (Recipe, error) {
	ids := []string{}
	for _, g := range r.Ingredients {
		for _, e := range g.Entries {
			if e.IngredientID != "" {
				ids = append(ids, e.IngredientID)
			}
		}
	}
	if len(ids) == 0 {
		return r, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT id,name,category FROM ingredient_catalog WHERE id=ANY($1::text[])`, ids)
	if err != nil {
		return r, err
	}
	defer rows.Close()
	known := map[string]CatalogIngredient{}
	for rows.Next() {
		var c CatalogIngredient
		if err := rows.Scan(&c.ID, &c.Name, &c.Category); err != nil {
			return r, err
		}
		known[c.ID] = c
	}
	if err := rows.Err(); err != nil {
		return r, err
	}
	for gi := range r.Ingredients {
		g := &r.Ingredients[gi]
		for i := range g.Entries {
			e := &g.Entries[i]
			if c, ok := known[e.IngredientID]; ok {
				e.Name, e.Category = c.Name, c.Category
			}
		}
		if g.Entries != nil {
			if err := normalizeIngredients(g); err != nil {
				return r, err
			}
		}
	}
	return r, nil
}

// SaveCatalog creates or edits a shared ingredient, atomically reserving every
// canonical name and alias. Existing recipe snapshots keep their display name.
func (s *Service) SaveCatalog(ctx context.Context, c CatalogIngredient) (CatalogIngredient, error) {
	c.Name = strings.Join(strings.Fields(c.Name), " ")
	c.Category = strings.TrimSpace(c.Category)
	if c.Category == "" {
		c.Category = "Other"
	}
	if c.Name == "" || len(c.Name) > 200 || len(c.Category) > 100 || len(c.Aliases) > 50 {
		return c, invalid("provide an ingredient name (up to 200 characters), category (up to 100), and at most 50 aliases")
	}
	names := map[string]bool{strings.ToLower(c.Name): true}
	for _, alias := range c.Aliases {
		alias = strings.ToLower(strings.Join(strings.Fields(alias), " "))
		if len(alias) > 200 {
			return c, invalid("ingredient alias is too long")
		}
		if alias != "" {
			names[alias] = true
		}
	}
	keys := make([]string, 0, len(names))
	for name := range names {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return c, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('cookira ingredient catalog', 0))`); err != nil {
		return c, err
	}
	for _, name := range keys {
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, name); err != nil {
			return c, err
		}
		var owner string
		err = tx.QueryRow(ctx, `SELECT ingredient_id FROM ingredient_names WHERE name=$1`, name).Scan(&owner)
		if err == nil && owner != c.ID {
			return c, invalid("name or alias %q already belongs to another ingredient", name)
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return c, err
		}
	}
	if c.ID == "" {
		err = tx.QueryRow(ctx, `INSERT INTO ingredient_catalog(id,name,category) VALUES(gen_random_uuid()::text,$1,$2) RETURNING id`, c.Name, c.Category).Scan(&c.ID)
	} else {
		var id string
		err = tx.QueryRow(ctx, `UPDATE ingredient_catalog SET name=$2,category=$3 WHERE id=$1 RETURNING id`, c.ID, c.Name, c.Category).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return c, invalid("unknown ingredient id")
		}
	}
	if err != nil {
		return c, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM ingredient_names WHERE ingredient_id=$1`, c.ID); err != nil {
		return c, err
	}
	c.Aliases = []string{}
	for _, name := range keys {
		if _, err = tx.Exec(ctx, `INSERT INTO ingredient_names(name,ingredient_id) VALUES($1,$2)`, name, c.ID); err != nil {
			return c, err
		}
		if name != strings.ToLower(c.Name) {
			c.Aliases = append(c.Aliases, name)
		}
	}
	return c, tx.Commit(ctx)
}

// bindIngredients runs in the recipe transaction: failed saves cannot leave
// catalog entries behind. A name/alias is resolved before creating a new item.
func bindIngredients(ctx context.Context, q *db.Queries, r *Recipe) error {
	for gi := range r.Ingredients {
		g := &r.Ingredients[gi]
		for i := range g.Entries {
			e := &g.Entries[i]
			if e.Text != "" {
				continue
			}
			id, name, category, err := q.ResolveIngredient(ctx, e.IngredientID, e.Name)
			if errors.Is(err, pgx.ErrNoRows) {
				return invalid("unknown ingredient id %q", e.IngredientID)
			}
			if err != nil {
				return err
			}
			e.IngredientID, e.Name, e.Category = id, name, category
		}
		if g.Entries != nil {
			if err := normalizeIngredients(g); err != nil {
				return err
			}
		}
	}
	return nil
}
