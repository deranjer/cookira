package recipes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/deranjer/cookira/internal/db"
	"github.com/deranjer/cookira/internal/storage"
)

// testService connects to TEST_DATABASE_URL (a throwaway Postgres 18
// database) and skips when it is not set. Each test works on uniquely named
// recipes, so tests do not need an empty database.
func testService(t *testing.T) *Service {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	if err := db.Migrate(url); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	store, err := storage.NewDisk(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return New(pool, db.New(pool), store, "https://cook.example")
}

func uniq(prefix string) string { return fmt.Sprintf("%s %d", prefix, time.Now().UnixNano()) }

func ptr[T any](v T) *T { return &v }

func TestRecipeLifecycle(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	name := uniq("Lifecycle")

	created, err := svc.Create(ctx, Fields{
		Title: ptr(name), Tags: &[]string{"Test"},
		Ingredients: &[]Group{{Name: "Main", Items: []string{"1 carrot"}}},
		Steps:       &[]Step{{Text: "Chop"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != StatusDraft {
		t.Errorf("status = %q, want draft by default", created.Status)
	}

	// A second recipe with the same title gets a distinct slug.
	dup, err := svc.Create(ctx, Fields{Title: ptr(name)})
	if err != nil {
		t.Fatal(err)
	}
	if dup.Slug == created.Slug {
		t.Errorf("duplicate title reused slug %q", dup.Slug)
	}

	// Patch by slug leaves other fields alone.
	updated, err := svc.Update(ctx, created.Slug, Fields{Servings: ptr(6)})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Servings != 6 || len(updated.Steps) != 1 || updated.Title != name {
		t.Errorf("patch result unexpected: %#v", updated)
	}

	// Structural helpers.
	withStep, err := svc.AddStep(ctx, created.ID, Step{Text: "Wash"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(withStep.Steps) != 2 || withStep.Steps[0].Text != "Wash" {
		t.Errorf("AddStep position 1 wrong: %#v", withStep.Steps)
	}
	withIng, err := svc.AddIngredient(ctx, created.ID, "main", "1 onion")
	if err != nil {
		t.Fatal(err)
	}
	if len(withIng.Ingredients) != 1 || len(withIng.Ingredients[0].Items) != 2 {
		t.Errorf("AddIngredient should reuse the group case-insensitively: %#v", withIng.Ingredients)
	}

	// Search finds it by ingredient.
	found, _, err := svc.List(ctx, ListParams{Query: "onion", Tag: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if !containsID(found, created.ID) {
		t.Error("search for ingredient did not find the recipe")
	}

	// History, restore.
	revs, err := svc.Revisions(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(revs) != 4 { // create + update + add step + add ingredient
		t.Fatalf("revisions = %d, want 4", len(revs))
	}
	restored, err := svc.Restore(ctx, created.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Servings != 0 || len(restored.Steps) != 1 || restored.Slug != created.Slug {
		t.Errorf("restore to revision 1 wrong: %#v", restored)
	}

	// Soft delete, then restore by id.
	if _, err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after delete = %v, want ErrNotFound", err)
	}
	back, err := svc.Restore(ctx, created.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if back.ID != created.ID {
		t.Error("restore returned a different recipe")
	}
	if _, err := svc.Get(ctx, created.Slug); err != nil {
		t.Errorf("Get after undelete: %v", err)
	}
}

func TestShareLinks(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	r, err := svc.Create(ctx, Fields{Title: ptr(uniq("Shared"))})
	if err != nil {
		t.Fatal(err)
	}
	sh, err := svc.CreateShare(ctx, r.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(sh.URL, "https://cook.example/s/") || len(sh.Token) < 30 {
		t.Errorf("unexpected share: %#v", sh)
	}
	got, err := svc.GetShared(ctx, sh.Token)
	if err != nil || got.ID != r.ID {
		t.Fatalf("GetShared = %v, %v", got.ID, err)
	}
	if err := svc.RevokeShare(ctx, r.ID, sh.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetShared(ctx, sh.Token); !errors.Is(err, ErrNotFound) {
		t.Errorf("revoked token still resolves: %v", err)
	}
	if _, err := svc.GetShared(ctx, "no-such-token"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown token = %v, want ErrNotFound", err)
	}

	// Deleting the recipe also kills its links.
	sh2, _ := svc.CreateShare(ctx, r.ID, 1)
	if _, err := svc.Delete(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetShared(ctx, sh2.Token); !errors.Is(err, ErrNotFound) {
		t.Errorf("share of deleted recipe resolves: %v", err)
	}
}

func containsID(list []Summary, id string) bool {
	for _, s := range list {
		if s.ID == id {
			return true
		}
	}
	return false
}

func TestImageSetOpenRemove(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	r, err := svc.Create(ctx, Fields{Title: ptr(uniq("Photo"))})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetImage(ctx, r.ID, []byte("not an image")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("SetImage(garbage) = %v, want ErrInvalid", err)
	}
	src := encodePNGBytes(t, 2000, 1000)
	if err := svc.SetImage(ctx, r.ID, src); err != nil {
		t.Fatal(err)
	}
	got, _ := svc.Get(ctx, r.ID)
	if !got.HasImage {
		t.Fatal("HasImage false after SetImage")
	}
	sizes := map[bool]int{}
	for _, thumb := range []bool{false, true} {
		f, ctype, err := svc.OpenImage(ctx, r.ID, thumb)
		if err != nil || ctype != "image/jpeg" {
			t.Fatalf("OpenImage(thumb=%v) = %v, %v", thumb, ctype, err)
		}
		data, _ := io.ReadAll(f)
		f.Close()
		sizes[thumb] = len(data)
	}
	if sizes[true] >= sizes[false] {
		t.Errorf("thumb %d bytes not smaller than full %d", sizes[true], sizes[false])
	}
	// Replacing swaps the files rather than leaving orphans behind.
	if err := svc.SetImage(ctx, r.ID, src); err != nil {
		t.Fatal(err)
	}
	if err := svc.RemoveImage(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ = svc.Get(ctx, r.ID); got.HasImage {
		t.Error("HasImage true after RemoveImage")
	}
	if _, _, err := svc.OpenImage(ctx, r.ID, false); !errors.Is(err, ErrNotFound) {
		t.Errorf("OpenImage after remove = %v, want ErrNotFound", err)
	}
}

func encodePNGBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
