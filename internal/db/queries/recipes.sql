-- name: CreateRecipe :one
INSERT INTO recipes (
    slug, title, tagline, author, servings, prep_minutes, cook_minutes, calories,
    source_url, notes, status, tags, ingredients, steps, created_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
)
RETURNING *;

-- name: GetRecipe :one
SELECT * FROM recipes WHERE id = $1 AND deleted_at IS NULL;

-- name: GetRecipeBySlug :one
SELECT * FROM recipes WHERE slug = $1 AND deleted_at IS NULL;

-- name: GetRecipeForUpdate :one
SELECT * FROM recipes WHERE id = $1 FOR UPDATE;

-- name: SlugExists :one
SELECT EXISTS (SELECT 1 FROM recipes WHERE slug = $1);

-- name: UpdateRecipe :one
UPDATE recipes SET
    slug = $2, title = $3, tagline = $4, author = $5, servings = $6,
    prep_minutes = $7, cook_minutes = $8, calories = $9, source_url = $10,
    notes = $11, status = $12, tags = $13, ingredients = $14, steps = $15,
    deleted_at = $16, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetRecipeImage :exec
UPDATE recipes SET image_key = $2, image_thumb_key = $3, image_type = $4, updated_at = now() WHERE id = $1;

-- name: ListRecipes :many
SELECT id, slug, title, tagline, author, servings, prep_minutes, cook_minutes,
       calories, status, tags, image_key, created_at, updated_at
FROM recipes
WHERE deleted_at IS NULL
  AND (sqlc.arg(status)::text = '' OR status = sqlc.arg(status)::text)
  AND (sqlc.arg(tag)::text = '' OR sqlc.arg(tag)::text = ANY(tags))
  AND (
    sqlc.arg(q)::text = ''
    OR search @@ websearch_to_tsquery('english', sqlc.arg(q)::text)
    OR title ILIKE '%' || sqlc.arg(q)::text || '%'
  )
ORDER BY
  CASE WHEN sqlc.arg(q)::text = '' THEN 0
       ELSE ts_rank(search, websearch_to_tsquery('english', sqlc.arg(q)::text)) END DESC,
  updated_at DESC
LIMIT sqlc.arg(lim)::int OFFSET sqlc.arg(off)::int;

-- name: CountRecipes :one
SELECT count(*) FROM recipes
WHERE deleted_at IS NULL
  AND (sqlc.arg(status)::text = '' OR status = sqlc.arg(status)::text)
  AND (sqlc.arg(tag)::text = '' OR sqlc.arg(tag)::text = ANY(tags))
  AND (
    sqlc.arg(q)::text = ''
    OR search @@ websearch_to_tsquery('english', sqlc.arg(q)::text)
    OR title ILIKE '%' || sqlc.arg(q)::text || '%'
  );

-- name: ListTags :many
SELECT tag::text AS tag, count(*) AS count
FROM recipes, unnest(tags) AS tag
WHERE deleted_at IS NULL
GROUP BY tag
ORDER BY count(*) DESC, tag;

-- name: NextRevision :one
SELECT (COALESCE(MAX(revision), 0) + 1)::int FROM recipe_revisions WHERE recipe_id = $1;

-- name: CreateRevision :exec
INSERT INTO recipe_revisions (recipe_id, revision, action, actor, user_id, snapshot)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListRevisions :many
SELECT id, recipe_id, revision, action, actor, created_at
FROM recipe_revisions WHERE recipe_id = $1 ORDER BY revision DESC;

-- name: GetRevision :one
SELECT * FROM recipe_revisions WHERE recipe_id = $1 AND revision = $2;

-- name: CreateShare :one
INSERT INTO recipe_shares (recipe_id, token, created_by, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListShares :many
SELECT * FROM recipe_shares WHERE recipe_id = $1 AND revoked_at IS NULL ORDER BY created_at DESC;

-- name: RevokeShare :execrows
UPDATE recipe_shares SET revoked_at = now()
WHERE id = $1 AND recipe_id = $2 AND revoked_at IS NULL;

-- name: GetRecipeByShareToken :one
SELECT r.* FROM recipe_shares s
JOIN recipes r ON r.id = s.recipe_id
WHERE s.token = $1
  AND s.revoked_at IS NULL
  AND (s.expires_at IS NULL OR s.expires_at > now())
  AND r.deleted_at IS NULL;

-- name: GetRecipeAny :one
SELECT * FROM recipes WHERE id = $1;
