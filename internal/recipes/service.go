package recipes

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/deranjer/cookira/internal/auth"
	"github.com/deranjer/cookira/internal/db"
	"github.com/deranjer/cookira/internal/storage"
)

type Service struct {
	pool      *pgxpool.Pool
	q         *db.Queries
	store     storage.Store
	publicURL string
	fetcher   *fetcher
}

// New builds the service. publicURL is the externally visible base URL
// (e.g. https://cookira.example.com), used to build share links.
func New(pool *pgxpool.Pool, q *db.Queries, store storage.Store, publicURL string) *Service {
	return &Service{pool: pool, q: q, store: store, publicURL: strings.TrimRight(publicURL, "/"), fetcher: newFetcher(false)}
}

type ListParams struct {
	Query  string
	Tag    string
	Status string
	Limit  int
	Offset int
}

func (s *Service) List(ctx context.Context, p ListParams) ([]Summary, int, error) {
	if p.Limit <= 0 || p.Limit > 200 {
		p.Limit = 50
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	if p.Status != "" && p.Status != StatusDraft && p.Status != StatusPublished {
		return nil, 0, invalid("status must be %q or %q", StatusDraft, StatusPublished)
	}
	q := strings.TrimSpace(p.Query)
	tag := strings.ToLower(strings.TrimSpace(p.Tag))
	rows, err := s.q.ListRecipes(ctx, db.ListRecipesParams{
		Status: p.Status, Tag: tag, Q: q, Lim: int32(p.Limit), Off: int32(p.Offset),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountRecipes(ctx, db.CountRecipesParams{Status: p.Status, Tag: tag, Q: q})
	if err != nil {
		return nil, 0, err
	}
	out := make([]Summary, len(rows))
	for i, r := range rows {
		out[i] = summaryFromRow(r)
	}
	return out, int(total), nil
}

// Get resolves ref (a uuid or a slug) to a live recipe.
func (s *Service) Get(ctx context.Context, ref string) (Recipe, error) {
	row, err := s.resolve(ctx, s.q, ref, false)
	if err != nil {
		return Recipe{}, err
	}
	return fromRow(row)
}

func (s *Service) resolve(ctx context.Context, q *db.Queries, ref string, allowDeleted bool) (db.Recipe, error) {
	ref = strings.TrimSpace(ref)
	var (
		row db.Recipe
		err error
	)
	if id, ok := uuidOf(ref); ok {
		if allowDeleted {
			row, err = q.GetRecipeAny(ctx, id)
		} else {
			row, err = q.GetRecipe(ctx, id)
		}
	} else {
		row, err = q.GetRecipeBySlug(ctx, ref)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Recipe{}, ErrNotFound
	}
	return row, err
}

func (s *Service) Create(ctx context.Context, f Fields) (Recipe, error) {
	r := Recipe{Status: StatusDraft}
	if err := apply(&r, f); err != nil {
		return Recipe{}, err
	}
	userID := currentUser(ctx)

	var out Recipe
	err := s.inTx(ctx, func(q *db.Queries) error {
		slug := r.Slug
		explicit := slug != ""
		if !explicit {
			slug = Slugify(r.Title)
		}
		slug, err := s.uniqueSlug(ctx, q, slug, explicit)
		if err != nil {
			return err
		}
		ing, steps, err := encode(r)
		if err != nil {
			return err
		}
		row, err := q.CreateRecipe(ctx, db.CreateRecipeParams{
			Slug: slug, Title: r.Title, Tagline: r.Tagline, Author: r.Author,
			Servings: int32(r.Servings), PrepMinutes: int32(r.PrepMinutes),
			CookMinutes: int32(r.CookMinutes), Calories: int32(r.Calories),
			SourceUrl: r.SourceURL, Notes: r.Notes, Status: r.Status, Tags: r.Tags,
			Ingredients: ing, Steps: steps, CreatedBy: userID,
		})
		if err != nil {
			return err
		}
		if out, err = fromRow(row); err != nil {
			return err
		}
		return s.writeRevision(ctx, q, row.ID, "create", out, userID)
	})
	return out, err
}

// Update applies f as a patch.
func (s *Service) Update(ctx context.Context, ref string, f Fields) (Recipe, error) {
	return s.mutate(ctx, ref, "update", false, func(r *Recipe, _ *bool) error {
		return apply(r, f)
	})
}

func (s *Service) Delete(ctx context.Context, ref string) (Recipe, error) {
	return s.mutate(ctx, ref, "delete", false, func(_ *Recipe, deleted *bool) error {
		*deleted = true
		return nil
	})
}

// AddStep inserts a step at position (1-based; 0 or past the end appends).
func (s *Service) AddStep(ctx context.Context, ref string, step Step, position int) (Recipe, error) {
	return s.mutate(ctx, ref, "update", false, func(r *Recipe, _ *bool) error {
		i := len(r.Steps)
		if position > 0 && position-1 < len(r.Steps) {
			i = position - 1
		}
		r.Steps = append(r.Steps[:i], append([]Step{step}, r.Steps[i:]...)...)
		return normalize(r)
	})
}

// AddIngredient appends text to the named ingredient group, creating the
// group if needed. An empty group name targets the unnamed group.
func (s *Service) AddIngredient(ctx context.Context, ref, group, text string) (Recipe, error) {
	group = strings.TrimSpace(group)
	return s.mutate(ctx, ref, "update", false, func(r *Recipe, _ *bool) error {
		for i := range r.Ingredients {
			if strings.EqualFold(r.Ingredients[i].Name, group) {
				r.Ingredients[i].Items = append(r.Ingredients[i].Items, text)
				return normalize(r)
			}
		}
		r.Ingredients = append(r.Ingredients, Group{Name: group, Items: []string{text}})
		return normalize(r)
	})
}

// Revisions lists a recipe's change history, newest first. Works for
// soft-deleted recipes when ref is a uuid.
func (s *Service) Revisions(ctx context.Context, ref string) ([]Revision, error) {
	row, err := s.resolve(ctx, s.q, ref, true)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListRevisions(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	out := make([]Revision, len(rows))
	for i, r := range rows {
		out[i] = Revision{
			Revision: int(r.Revision), Action: r.Action, Actor: r.Actor,
			CreatedAt: r.CreatedAt.Time.Format(timeFormat),
		}
	}
	return out, nil
}

// Restore rewrites a recipe's content from an earlier revision (and
// un-deletes it if it was deleted). The slug is kept.
func (s *Service) Restore(ctx context.Context, ref string, revision int) (Recipe, error) {
	return s.mutate(ctx, ref, "restore", true, func(r *Recipe, deleted *bool) error {
		row, err := s.q.GetRevision(ctx, db.GetRevisionParams{RecipeID: uuidMust(r.ID), Revision: int32(revision)})
		if errors.Is(err, pgx.ErrNoRows) {
			return invalid("revision %d does not exist", revision)
		}
		if err != nil {
			return err
		}
		var snap Recipe
		if err := json.Unmarshal(row.Snapshot, &snap); err != nil {
			return fmt.Errorf("decoding revision: %w", err)
		}
		snap.ID, snap.Slug, snap.HasImage = r.ID, r.Slug, r.HasImage
		snap.CreatedAt, snap.UpdatedAt = r.CreatedAt, r.UpdatedAt
		*r = snap
		*deleted = false
		return normalize(r)
	})
}

func (s *Service) Tags(ctx context.Context) ([]TagCount, error) {
	rows, err := s.q.ListTags(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]TagCount, len(rows))
	for i, r := range rows {
		out[i] = TagCount{Tag: r.Tag, Count: int(r.Count)}
	}
	return out, nil
}

// mutate is the single write path for existing recipes: it locks the row,
// lets fn change it, persists it and records a revision, all in one
// transaction.
func (s *Service) mutate(ctx context.Context, ref, action string, allowDeleted bool, fn func(r *Recipe, deleted *bool) error) (Recipe, error) {
	userID := currentUser(ctx)
	var out Recipe
	err := s.inTx(ctx, func(q *db.Queries) error {
		found, err := s.resolve(ctx, q, ref, allowDeleted)
		if err != nil {
			return err
		}
		row, err := q.GetRecipeForUpdate(ctx, found.ID)
		if err != nil {
			return err
		}
		r, err := fromRow(row)
		if err != nil {
			return err
		}
		deleted := row.DeletedAt.Valid
		oldSlug := r.Slug
		if err := fn(&r, &deleted); err != nil {
			return err
		}
		if r.Slug != oldSlug {
			if r.Slug, err = s.uniqueSlug(ctx, q, r.Slug, true); err != nil {
				return err
			}
		}
		ing, steps, err := encode(r)
		if err != nil {
			return err
		}
		var deletedAt pgtype.Timestamptz
		if deleted {
			deletedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
			if row.DeletedAt.Valid {
				deletedAt = row.DeletedAt
			}
		}
		updated, err := q.UpdateRecipe(ctx, db.UpdateRecipeParams{
			ID: row.ID, Slug: r.Slug, Title: r.Title, Tagline: r.Tagline, Author: r.Author,
			Servings: int32(r.Servings), PrepMinutes: int32(r.PrepMinutes),
			CookMinutes: int32(r.CookMinutes), Calories: int32(r.Calories),
			SourceUrl: r.SourceURL, Notes: r.Notes, Status: r.Status, Tags: r.Tags,
			Ingredients: ing, Steps: steps, DeletedAt: deletedAt,
		})
		if err != nil {
			return err
		}
		if out, err = fromRow(updated); err != nil {
			return err
		}
		if deleted && !row.DeletedAt.Valid {
			action = "delete"
		}
		return s.writeRevision(ctx, q, row.ID, action, out, userID)
	})
	return out, err
}

func (s *Service) writeRevision(ctx context.Context, q *db.Queries, id pgtype.UUID, action string, r Recipe, userID pgtype.UUID) error {
	n, err := q.NextRevision(ctx, id)
	if err != nil {
		return err
	}
	snap, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return q.CreateRevision(ctx, db.CreateRevisionParams{
		RecipeID: id, Revision: n, Action: action, Actor: auth.Actor(ctx), UserID: userID, Snapshot: snap,
	})
}

func (s *Service) uniqueSlug(ctx context.Context, q *db.Queries, slug string, explicit bool) (string, error) {
	candidate := slug
	for i := 2; ; i++ {
		exists, err := q.SlugExists(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		if explicit {
			return "", invalid("slug %q is already in use", slug)
		}
		candidate = fmt.Sprintf("%s-%d", slug, i)
	}
}

func (s *Service) inTx(ctx context.Context, fn func(q *db.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(s.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func encode(r Recipe) (ingredients, steps []byte, err error) {
	if ingredients, err = json.Marshal(r.Ingredients); err != nil {
		return nil, nil, err
	}
	if steps, err = json.Marshal(r.Steps); err != nil {
		return nil, nil, err
	}
	return ingredients, steps, nil
}

func currentUser(ctx context.Context) pgtype.UUID {
	if idStr, ok := auth.UserID(ctx); ok {
		if u, ok := uuidOf(idStr); ok {
			return u
		}
	}
	return pgtype.UUID{}
}

func uuidMust(s string) pgtype.UUID {
	u, _ := uuidOf(s)
	return u
}

// ---- Share links ----

// CreateShare mints a view-only link. expiresInDays <= 0 means no expiry.
func (s *Service) CreateShare(ctx context.Context, ref string, expiresInDays int) (Share, error) {
	row, err := s.resolve(ctx, s.q, ref, false)
	if err != nil {
		return Share{}, err
	}
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return Share{}, err
	}
	var expires pgtype.Timestamptz
	if expiresInDays > 0 {
		expires = pgtype.Timestamptz{Time: time.Now().AddDate(0, 0, expiresInDays), Valid: true}
	}
	sh, err := s.q.CreateShare(ctx, db.CreateShareParams{
		RecipeID:  row.ID,
		Token:     base64.RawURLEncoding.EncodeToString(b),
		CreatedBy: currentUser(ctx),
		ExpiresAt: expires,
	})
	if err != nil {
		return Share{}, err
	}
	return s.shareOut(sh), nil
}

func (s *Service) ListShares(ctx context.Context, ref string) ([]Share, error) {
	row, err := s.resolve(ctx, s.q, ref, false)
	if err != nil {
		return nil, err
	}
	shares, err := s.q.ListShares(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	out := make([]Share, len(shares))
	for i, sh := range shares {
		out[i] = s.shareOut(sh)
	}
	return out, nil
}

func (s *Service) RevokeShare(ctx context.Context, ref, shareID string) error {
	row, err := s.resolve(ctx, s.q, ref, false)
	if err != nil {
		return err
	}
	id, ok := uuidOf(shareID)
	if !ok {
		return invalid("invalid share id")
	}
	n, err := s.q.RevokeShare(ctx, db.RevokeShareParams{ID: id, RecipeID: row.ID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) shareOut(sh db.RecipeShare) Share {
	out := Share{
		ID: sh.ID.String(), Token: sh.Token, URL: s.publicURL + "/s/" + sh.Token,
		CreatedAt: sh.CreatedAt.Time.Format(timeFormat),
	}
	if sh.ExpiresAt.Valid {
		t := sh.ExpiresAt.Time.Format(timeFormat)
		out.ExpiresAt = &t
	}
	return out
}

// GetShared resolves a share token for the public, unauthenticated view.
func (s *Service) GetShared(ctx context.Context, token string) (Recipe, error) {
	row, err := s.q.GetRecipeByShareToken(ctx, token)
	if errors.Is(err, pgx.ErrNoRows) {
		return Recipe{}, ErrNotFound
	}
	if err != nil {
		return Recipe{}, err
	}
	return fromRow(row)
}

// ---- Images ----

// MaxImageBytes caps the size of an uploaded or fetched source image.
const MaxImageBytes = 10 << 20

// SetImage validates, resizes and stores data as the recipe's header image,
// replacing any existing one. A full-size and a thumbnail JPEG are kept.
func (s *Service) SetImage(ctx context.Context, ref string, data []byte) error {
	if len(data) > MaxImageBytes {
		return invalid("image is larger than %d MB", MaxImageBytes>>20)
	}
	row, err := s.resolve(ctx, s.q, ref, false)
	if err != nil {
		return err
	}
	full, thumb, err := processImage(data)
	if err != nil {
		return err
	}
	rnd := make([]byte, 8)
	if _, err := rand.Read(rnd); err != nil {
		return err
	}
	base := fmt.Sprintf("recipes/%s/%x", row.ID.String(), rnd)
	fullKey, thumbKey := base+".jpg", base+"-thumb.jpg"
	if _, err := s.store.Save(ctx, fullKey, bytes.NewReader(full)); err != nil {
		return err
	}
	if _, err := s.store.Save(ctx, thumbKey, bytes.NewReader(thumb)); err != nil {
		_ = s.store.Delete(ctx, fullKey)
		return err
	}
	if err := s.q.SetRecipeImage(ctx, db.SetRecipeImageParams{
		ID: row.ID, ImageKey: fullKey, ImageThumbKey: thumbKey, ImageType: "image/jpeg",
	}); err != nil {
		_ = s.store.Delete(ctx, fullKey)
		_ = s.store.Delete(ctx, thumbKey)
		return err
	}
	s.deleteImageFiles(ctx, row)
	return nil
}

// SetImageFromURL downloads an image (http/https only, public hosts only)
// and stores it like SetImage.
func (s *Service) SetImageFromURL(ctx context.Context, ref, url string) error {
	data, err := s.fetcher.fetch(ctx, url)
	if err != nil {
		return err
	}
	return s.SetImage(ctx, ref, data)
}

// RemoveImage deletes the recipe's header image.
func (s *Service) RemoveImage(ctx context.Context, ref string) error {
	row, err := s.resolve(ctx, s.q, ref, false)
	if err != nil {
		return err
	}
	if row.ImageKey == "" {
		return nil
	}
	if err := s.q.SetRecipeImage(ctx, db.SetRecipeImageParams{ID: row.ID}); err != nil {
		return err
	}
	s.deleteImageFiles(ctx, row)
	return nil
}

func (s *Service) deleteImageFiles(ctx context.Context, row db.Recipe) {
	for _, key := range []string{row.ImageKey, row.ImageThumbKey} {
		if key != "" {
			_ = s.store.Delete(ctx, key)
		}
	}
}

// OpenImage streams the image of a live recipe (the thumbnail if thumb).
func (s *Service) OpenImage(ctx context.Context, ref string, thumb bool) (io.ReadCloser, string, error) {
	row, err := s.resolve(ctx, s.q, ref, false)
	if err != nil {
		return nil, "", err
	}
	return s.openImageRow(ctx, row, thumb)
}

// OpenSharedImage streams the image of a recipe reachable via a share token.
func (s *Service) OpenSharedImage(ctx context.Context, token string, thumb bool) (io.ReadCloser, string, error) {
	row, err := s.q.GetRecipeByShareToken(ctx, token)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	return s.openImageRow(ctx, row, thumb)
}

func (s *Service) openImageRow(ctx context.Context, row db.Recipe, thumb bool) (io.ReadCloser, string, error) {
	key := row.ImageKey
	if thumb && row.ImageThumbKey != "" {
		key = row.ImageThumbKey
	}
	if key == "" {
		return nil, "", ErrNotFound
	}
	f, err := s.store.Open(ctx, key)
	if err != nil {
		return nil, "", ErrNotFound
	}
	return f, row.ImageType, nil
}
