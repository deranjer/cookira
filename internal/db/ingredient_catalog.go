package db

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ResolveIngredient shares the transaction used by recipe writes. The names
// table is the unique namespace for both canonical names and aliases.
func (q *Queries) ResolveIngredient(ctx context.Context, id, name string) (string, string, string, error) {
	if _, err := q.db.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('cookira ingredient catalog', 0))`); err != nil {
		return "", "", "", err
	}
	var canonical, category string
	if id != "" {
		err := q.db.QueryRow(ctx, `SELECT id, name, category FROM ingredient_catalog WHERE id=$1`, id).Scan(&id, &canonical, &category)
		return id, canonical, category, err
	}
	key := strings.ToLower(strings.Join(strings.Fields(name), " "))
	// Lock the normalized name before lookup/create so concurrent aliases and
	// new names always resolve to one record without orphan catalog entries.
	if _, err := q.db.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, key); err != nil {
		return "", "", "", err
	}
	err := q.db.QueryRow(ctx, `SELECT c.id, c.name, c.category FROM ingredient_names n JOIN ingredient_catalog c ON c.id=n.ingredient_id WHERE n.name=$1`, key).Scan(&id, &canonical, &category)
	if err == nil {
		return id, canonical, category, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", err
	}
	err = q.db.QueryRow(ctx, `INSERT INTO ingredient_catalog(id,name) VALUES(gen_random_uuid()::text,$1) RETURNING id,name,category`, strings.Join(strings.Fields(name), " ")).Scan(&id, &canonical, &category)
	if err != nil {
		return "", "", "", err
	}
	_, err = q.db.Exec(ctx, `INSERT INTO ingredient_names(name,ingredient_id) VALUES($1,$2)`, key, id)
	return id, canonical, category, err
}
