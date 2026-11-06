// Package recipes is the domain layer for recipes: validation, revisions,
// share links and images. Both the REST API (internal/api) and the MCP
// server (internal/mcpserver) go through Service, so the two surfaces can't
// drift apart in behavior.
package recipes

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/deranjer/cookira/internal/db"
)

var (
	ErrNotFound = errors.New("recipe not found")
	// ErrInvalid wraps every validation failure; the message is safe to show
	// to a user or an agent.
	ErrInvalid = errors.New("invalid recipe")
)

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

const (
	StatusDraft     = "draft"
	StatusPublished = "published"

	maxTitle    = 200
	maxGroups   = 30
	maxItems    = 200
	maxSteps    = 200
	maxSubsteps = 50
	maxTags     = 30
	maxTextLen  = 4000
	maxNotesLen = 20000
	timeFormat  = time.RFC3339
	maxSlugLen  = 80
)

// Group is a named section of ingredients ("Broth", "Noodles"). An unnamed
// group (Name == "") is the recipe's plain ingredient list.
type Group struct {
	Name  string   `json:"name"`
	Items []string `json:"items"`
}

// Step is one numbered direction, optionally with lettered sub-steps.
type Step struct {
	Text     string   `json:"text"`
	Substeps []string `json:"substeps,omitempty"`
}

// ImageFraming says how the header photo is framed. The stored image is never
// cropped: clients show it with object-position at the focal point and scale
// it by Zoom around that point. The zero value is not valid; use
// defaultFraming (centred, no zoom). Old snapshots without these fields are
// repaired by normalizeFraming.
type ImageFraming struct {
	ImageFocusX float64 `json:"imageFocusX" minimum:"0" maximum:"1" doc:"Focal point, 0 (left) to 1 (right)"`
	ImageFocusY float64 `json:"imageFocusY" minimum:"0" maximum:"1" doc:"Focal point, 0 (top) to 1 (bottom)"`
	ImageZoom   float64 `json:"imageZoom" minimum:"1" maximum:"4" doc:"Zoom factor around the focal point, 1 = whole photo fits the frame"`
}

const (
	maxZoom = 4.0
)

var defaultFraming = ImageFraming{ImageFocusX: 0.5, ImageFocusY: 0.5, ImageZoom: 1}

// normalizeFraming fills in defaults for a missing (zero) zoom, which is what
// revision snapshots taken before framing existed decode to.
func normalizeFraming(f *ImageFraming) {
	if f.ImageZoom == 0 {
		*f = defaultFraming
	}
}

func validFraming(f ImageFraming) bool {
	in01 := func(v float64) bool { return v >= 0 && v <= 1 } // also rejects NaN
	return in01(f.ImageFocusX) && in01(f.ImageFocusY) && f.ImageZoom >= 1 && f.ImageZoom <= maxZoom
}

// Recipe is the full API shape of a recipe. Numeric fields use 0 for
// "not specified".
type Recipe struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Tagline     string   `json:"tagline"`
	Author      string   `json:"author"`
	Servings    int      `json:"servings"`
	PrepMinutes int      `json:"prepMinutes"`
	CookMinutes int      `json:"cookMinutes"`
	Calories    int      `json:"calories"`
	SourceURL   string   `json:"sourceUrl"`
	Notes       string   `json:"notes"`
	Status      string   `json:"status"`
	Tags        []string `json:"tags"`
	Ingredients []Group  `json:"ingredients"`
	Steps       []Step   `json:"steps"`
	HasImage    bool     `json:"hasImage"`
	ImageFraming
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// Summary is the compact list shape (no ingredients/steps/notes).
type Summary struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Tagline     string   `json:"tagline"`
	Author      string   `json:"author"`
	Servings    int      `json:"servings"`
	PrepMinutes int      `json:"prepMinutes"`
	CookMinutes int      `json:"cookMinutes"`
	Calories    int      `json:"calories"`
	Status      string   `json:"status"`
	Tags        []string `json:"tags"`
	HasImage    bool     `json:"hasImage"`
	ImageFraming
	UpdatedAt string `json:"updatedAt"`
}

// Fields carries the editable parts of a recipe. A nil pointer means "leave
// unchanged" on update (and "use the default" on create), which gives agents
// real PATCH semantics.
type Fields struct {
	Slug        *string
	Title       *string
	Tagline     *string
	Author      *string
	Servings    *int
	PrepMinutes *int
	CookMinutes *int
	Calories    *int
	SourceURL   *string
	Notes       *string
	Status      *string
	Tags        *[]string
	Ingredients *[]Group
	Steps       *[]Step
	// Photo framing; each part is patched independently.
	ImageFocusX *float64
	ImageFocusY *float64
	ImageZoom   *float64
}

type Revision struct {
	Revision  int    `json:"revision"`
	Action    string `json:"action"`
	Actor     string `json:"actor"`
	CreatedAt string `json:"createdAt"`
}

type Share struct {
	ID        string  `json:"id"`
	Token     string  `json:"token"`
	URL       string  `json:"url"`
	CreatedAt string  `json:"createdAt"`
	ExpiresAt *string `json:"expiresAt"`
}

type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

func fromRow(r db.Recipe) (Recipe, error) {
	out := Recipe{
		ID:           r.ID.String(),
		Slug:         r.Slug,
		Title:        r.Title,
		Tagline:      r.Tagline,
		Author:       r.Author,
		Servings:     int(r.Servings),
		PrepMinutes:  int(r.PrepMinutes),
		CookMinutes:  int(r.CookMinutes),
		Calories:     int(r.Calories),
		SourceURL:    r.SourceUrl,
		Notes:        r.Notes,
		Status:       r.Status,
		Tags:         nonNil(r.Tags),
		HasImage:     r.ImageKey != "",
		ImageFraming: ImageFraming{ImageFocusX: r.ImageFocusX, ImageFocusY: r.ImageFocusY, ImageZoom: r.ImageZoom},
		CreatedAt:    r.CreatedAt.Time.Format(timeFormat),
		UpdatedAt:    r.UpdatedAt.Time.Format(timeFormat),
	}
	if err := json.Unmarshal(r.Ingredients, &out.Ingredients); err != nil {
		return Recipe{}, fmt.Errorf("decoding ingredients: %w", err)
	}
	if err := json.Unmarshal(r.Steps, &out.Steps); err != nil {
		return Recipe{}, fmt.Errorf("decoding steps: %w", err)
	}
	if out.Ingredients == nil {
		out.Ingredients = []Group{}
	}
	if out.Steps == nil {
		out.Steps = []Step{}
	}
	return out, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func summaryFromRow(r db.ListRecipesRow) Summary {
	return Summary{
		ID:           r.ID.String(),
		Slug:         r.Slug,
		Title:        r.Title,
		Tagline:      r.Tagline,
		Author:       r.Author,
		Servings:     int(r.Servings),
		PrepMinutes:  int(r.PrepMinutes),
		CookMinutes:  int(r.CookMinutes),
		Calories:     int(r.Calories),
		Status:       r.Status,
		Tags:         nonNil(r.Tags),
		HasImage:     r.ImageKey != "",
		ImageFraming: ImageFraming{ImageFocusX: r.ImageFocusX, ImageFocusY: r.ImageFocusY, ImageZoom: r.ImageZoom},
		UpdatedAt:    r.UpdatedAt.Time.Format(timeFormat),
	}
}

// apply merges f into r (r is modified in place) and normalizes/validates
// the result.
func apply(r *Recipe, f Fields) error {
	if f.Slug != nil {
		r.Slug = strings.TrimSpace(*f.Slug)
	}
	if f.Title != nil {
		r.Title = strings.TrimSpace(*f.Title)
	}
	if f.Tagline != nil {
		r.Tagline = strings.TrimSpace(*f.Tagline)
	}
	if f.Author != nil {
		r.Author = strings.TrimSpace(*f.Author)
	}
	if f.Servings != nil {
		r.Servings = *f.Servings
	}
	if f.PrepMinutes != nil {
		r.PrepMinutes = *f.PrepMinutes
	}
	if f.CookMinutes != nil {
		r.CookMinutes = *f.CookMinutes
	}
	if f.Calories != nil {
		r.Calories = *f.Calories
	}
	if f.SourceURL != nil {
		r.SourceURL = strings.TrimSpace(*f.SourceURL)
	}
	if f.Notes != nil {
		r.Notes = strings.TrimSpace(*f.Notes)
	}
	if f.Status != nil {
		r.Status = strings.TrimSpace(*f.Status)
	}
	if f.Tags != nil {
		r.Tags = *f.Tags
	}
	if f.Ingredients != nil {
		r.Ingredients = *f.Ingredients
	}
	if f.Steps != nil {
		r.Steps = *f.Steps
	}
	if f.ImageFocusX != nil {
		r.ImageFocusX = *f.ImageFocusX
	}
	if f.ImageFocusY != nil {
		r.ImageFocusY = *f.ImageFocusY
	}
	if f.ImageZoom != nil {
		r.ImageZoom = *f.ImageZoom
	}
	return normalize(r)
}

func normalize(r *Recipe) error {
	normalizeFraming(&r.ImageFraming)
	if !validFraming(r.ImageFraming) {
		return invalid("imageFocusX and imageFocusY must be between 0 and 1, and imageZoom between 1 and %g", maxZoom)
	}
	if r.Title == "" {
		return invalid("title is required")
	}
	if len(r.Title) > maxTitle {
		return invalid("title is longer than %d characters", maxTitle)
	}
	if r.Status != StatusDraft && r.Status != StatusPublished {
		return invalid("status must be %q or %q", StatusDraft, StatusPublished)
	}
	for name, v := range map[string]int{
		"servings": r.Servings, "prepMinutes": r.PrepMinutes,
		"cookMinutes": r.CookMinutes, "calories": r.Calories,
	} {
		if v < 0 || v > 1_000_000 {
			return invalid("%s must be between 0 and 1000000 (0 = not specified)", name)
		}
	}
	if len(r.Tagline) > maxTextLen || len(r.Author) > maxTextLen || len(r.SourceURL) > maxTextLen {
		return invalid("tagline, author and sourceUrl must each be under %d characters", maxTextLen)
	}
	if len(r.Notes) > maxNotesLen {
		return invalid("notes are longer than %d characters", maxNotesLen)
	}
	if r.SourceURL != "" && !strings.HasPrefix(r.SourceURL, "http://") && !strings.HasPrefix(r.SourceURL, "https://") {
		return invalid("sourceUrl must be an http(s) URL")
	}
	if r.Slug != "" {
		if len(r.Slug) > maxSlugLen || !slugRe.MatchString(r.Slug) {
			return invalid("slug must be lowercase letters, digits and single hyphens")
		}
	}

	// Tags: lowercase, trimmed, de-duplicated, order preserved.
	tags := make([]string, 0, len(r.Tags))
	for _, t := range r.Tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" || slices.Contains(tags, t) {
			continue
		}
		if len(t) > 60 {
			return invalid("tag %q is too long", t)
		}
		tags = append(tags, t)
	}
	if len(tags) > maxTags {
		return invalid("at most %d tags", maxTags)
	}
	r.Tags = tags

	groups := make([]Group, 0, len(r.Ingredients))
	for _, g := range r.Ingredients {
		items := cleanLines(g.Items)
		if len(items) == 0 {
			continue
		}
		if len(items) > maxItems {
			return invalid("at most %d ingredients per group", maxItems)
		}
		groups = append(groups, Group{Name: strings.TrimSpace(g.Name), Items: items})
	}
	if len(groups) > maxGroups {
		return invalid("at most %d ingredient groups", maxGroups)
	}
	r.Ingredients = groups

	steps := make([]Step, 0, len(r.Steps))
	for _, s := range r.Steps {
		text := strings.TrimSpace(s.Text)
		if text == "" {
			continue
		}
		if len(text) > maxTextLen {
			return invalid("a step is longer than %d characters", maxTextLen)
		}
		sub := cleanLines(s.Substeps)
		if len(sub) > maxSubsteps {
			return invalid("at most %d sub-steps per step", maxSubsteps)
		}
		steps = append(steps, Step{Text: text, Substeps: sub})
	}
	if len(steps) > maxSteps {
		return invalid("at most %d steps", maxSteps)
	}
	r.Steps = steps
	return nil
}

func cleanLines(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if len(s) > maxTextLen {
			s = s[:maxTextLen]
		}
		out = append(out, s)
	}
	return out
}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Slugify turns a title into a URL slug ("Chicken Ramen!" -> "chicken-ramen").
func Slugify(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			dash = false
		case b.Len() > 0 && !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > maxSlugLen {
		s = strings.Trim(s[:maxSlugLen], "-")
	}
	if s == "" {
		s = "recipe"
	}
	return s
}

func uuidOf(s string) (pgtype.UUID, bool) {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		return pgtype.UUID{}, false
	}
	return u, true
}
