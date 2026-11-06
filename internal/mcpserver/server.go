// Package mcpserver exposes Cookira's recipes to AI agents over the Model
// Context Protocol. It is mounted as a plain http.Handler at /mcp (see
// internal/api/router.go) behind OAuth: agents connect through the consent
// flow in internal/oauth and send the resulting Bearer access token. Every
// tool goes through recipes.Service, the same layer the REST
// API uses, so the two surfaces never drift apart.
//
// Agents are expected to create recipes as drafts; a human publishes them.
package mcpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/deranjer/cookira/internal/auth"
	"github.com/deranjer/cookira/internal/recipes"
	"github.com/deranjer/cookira/internal/version"
)

const instructions = `Cookira is a household recipe book. Recipes are structured: ingredients are
groups of plain-text lines (e.g. group "Broth": ["2 cups chicken stock", "1 tbsp soy sauce"]),
steps are numbered directions with optional sub-steps, and tags are lowercase words.
Search first (list_recipes with q) before creating, to avoid duplicates. New recipes are
created as drafts; leave status alone unless the user asks you to publish. Numbers use 0 for
"not specified". Every change is recorded in the recipe's history and can be restored.`

type toolServer struct {
	svc *recipes.Service
}

// New builds the MCP server and registers every tool.
func New(svc *recipes.Service) *mcp.Server {
	s := &toolServer{svc: svc}
	server := mcp.NewServer(
		&mcp.Implementation{Name: "cookira", Version: version.Version},
		&mcp.ServerOptions{Instructions: instructions},
	)

	addTool(server, &mcp.Tool{
		Name: "list_recipes",
		Description: "List recipes as compact summaries (no ingredients or steps; use get_recipe for those). " +
			"Use q for full-text search (title, tags, ingredients, notes), or filter by tag or status. " +
			"Page with limit/offset.",
	}, s.listRecipes)

	addTool(server, &mcp.Tool{
		Name:        "get_recipe",
		Description: "Get one recipe in full by id (uuid) or slug.",
	}, s.getRecipe)

	addTool(server, &mcp.Tool{
		Name:        "list_tags",
		Description: "List all tags in use with recipe counts. Reuse existing tags where they fit.",
	}, s.listTags)

	addTool(server, &mcp.Tool{
		Name:        "get_recipe_history",
		Description: "List every recorded change to a recipe (who, when, what kind). Use restore_recipe_revision to roll back.",
	}, s.getHistory)

	addTool(server, &mcp.Tool{
		Name: "create_recipe",
		Description: "Create a recipe (title is required). It is saved as a draft unless status is 'published'. " +
			"Requires read-write access.",
	}, s.createRecipe)

	addTool(server, &mcp.Tool{
		Name: "update_recipe",
		Description: "Update a recipe. Only the fields you supply change; omit the rest. " +
			"tags, ingredients and steps REPLACE the whole list when supplied (use add_recipe_step / " +
			"add_recipe_ingredient to append). Requires read-write access.",
	}, s.updateRecipe)

	addTool(server, &mcp.Tool{
		Name:        "add_recipe_step",
		Description: "Insert one step into a recipe at a 1-based position (omit or 0 to append). Requires read-write access.",
	}, s.addStep)

	addTool(server, &mcp.Tool{
		Name:        "add_recipe_ingredient",
		Description: "Append one ingredient line to a named ingredient group, creating the group if needed. Requires read-write access.",
	}, s.addIngredient)

	addTool(server, &mcp.Tool{
		Name: "delete_recipe",
		Description: "Delete a recipe. It is soft-deleted and can be brought back with restore_recipe_revision " +
			"using the recipe's id. Requires read-write access.",
	}, s.deleteRecipe)

	addTool(server, &mcp.Tool{
		Name: "restore_recipe_revision",
		Description: "Restore a recipe's content from an earlier revision number (see get_recipe_history). " +
			"Also un-deletes a deleted recipe (pass its id). Requires read-write access.",
	}, s.restoreRevision)

	addTool(server, &mcp.Tool{
		Name: "set_recipe_image",
		Description: "Set (or replace) a recipe's header photo, from an image URL or base64 data. " +
			"JPEG, PNG or WebP up to 10 MB; it is resized automatically and shown centred unless you pass focusX/focusY (0-1, where the subject is) and zoom (1-4). Give exactly one of imageUrl " +
			"(a public http/https link; private and local addresses are refused) or imageBase64. " +
			"Requires read-write access.",
	}, s.setImage)

	addTool(server, &mcp.Tool{
		Name: "create_share_link",
		Description: "Create a view-only public link to a recipe that works without logging in. " +
			"Only do this when the user asks. Requires read-write access.",
	}, s.createShare)

	addTool(server, &mcp.Tool{
		Name:        "list_share_links",
		Description: "List a recipe's active share links.",
	}, s.listShares)

	addTool(server, &mcp.Tool{
		Name:        "revoke_share_link",
		Description: "Revoke a share link by its id (from list_share_links). Requires read-write access.",
	}, s.revokeShare)

	return server
}

// addTool registers a typed tool whose result is a single compact-JSON text
// block. The SDK's default for a typed output is to send the payload twice
// (structuredContent plus a text copy) and to advertise an output schema; for
// an LLM client that is pure token overhead, so handlers keep typed
// signatures but the wire form is text only.
func addTool[In, Out any](server *mcp.Server, tool *mcp.Tool, h func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error)) {
	mcp.AddTool(server, tool, func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		res, out, err := h(ctx, req, in)
		if err != nil || res != nil {
			return res, nil, err
		}
		encoded, err := json.Marshal(out)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(encoded)}}}, nil, nil
	})
}

// errorResult reports a tool-level failure in the result content with
// IsError set (per the MCP spec), so the agent can read it and adjust.
func errorResult(format string, args ...any) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(format, args...)}},
	}
}

var errNoWrite = errorResult("this connection was approved as read-only and can't make changes")

// fail turns a service error into a tool result. Domain errors are shown to
// the agent verbatim; anything else is an internal error.
func fail[T any](err error) (*mcp.CallToolResult, T, error) {
	var zero T
	switch {
	case errors.Is(err, recipes.ErrNotFound):
		return errorResult("recipe not found"), zero, nil
	case errors.Is(err, recipes.ErrInvalid):
		return errorResult("%s", err), zero, nil
	default:
		return nil, zero, err
	}
}

// ---- read tools ----

type listArgs struct {
	Q      string `json:"q,omitempty" jsonschema:"full-text search over title, tags, ingredients and notes"`
	Tag    string `json:"tag,omitempty" jsonschema:"only recipes with this tag"`
	Status string `json:"status,omitempty" jsonschema:"draft or published; omit for both"`
	Limit  int    `json:"limit,omitempty" jsonschema:"max results, default 50, max 200"`
	Offset int    `json:"offset,omitempty" jsonschema:"results to skip, for paging"`
}

type listResult struct {
	Items []recipes.Summary `json:"items"`
	Total int               `json:"total"`
}

func (s *toolServer) listRecipes(ctx context.Context, _ *mcp.CallToolRequest, a listArgs) (*mcp.CallToolResult, listResult, error) {
	items, total, err := s.svc.List(ctx, recipes.ListParams{Query: a.Q, Tag: a.Tag, Status: a.Status, Limit: a.Limit, Offset: a.Offset})
	if err != nil {
		return fail[listResult](err)
	}
	return nil, listResult{Items: items, Total: total}, nil
}

type refArgs struct {
	Recipe string `json:"recipe" jsonschema:"recipe id (uuid) or slug"`
}

func (s *toolServer) getRecipe(ctx context.Context, _ *mcp.CallToolRequest, a refArgs) (*mcp.CallToolResult, recipes.Recipe, error) {
	r, err := s.svc.Get(ctx, a.Recipe)
	if err != nil {
		return fail[recipes.Recipe](err)
	}
	return nil, r, nil
}

func (s *toolServer) listTags(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, []recipes.TagCount, error) {
	tags, err := s.svc.Tags(ctx)
	if err != nil {
		return fail[[]recipes.TagCount](err)
	}
	return nil, tags, nil
}

func (s *toolServer) getHistory(ctx context.Context, _ *mcp.CallToolRequest, a refArgs) (*mcp.CallToolResult, []recipes.Revision, error) {
	revs, err := s.svc.Revisions(ctx, a.Recipe)
	if err != nil {
		return fail[[]recipes.Revision](err)
	}
	return nil, revs, nil
}

// ---- write tools ----

// recipeFields is the editable part of a recipe, shared by create and update.
type recipeFields struct {
	Title       *string         `json:"title,omitempty" jsonschema:"recipe title"`
	Slug        *string         `json:"slug,omitempty" jsonschema:"URL slug (lowercase-hyphenated); generated from the title if omitted"`
	Tagline     *string         `json:"tagline,omitempty" jsonschema:"one-sentence description"`
	Author      *string         `json:"author,omitempty" jsonschema:"who the recipe is credited to"`
	Servings    *int            `json:"servings,omitempty" jsonschema:"number of servings; 0 = not specified"`
	PrepMinutes *int            `json:"prepMinutes,omitempty" jsonschema:"prep time in minutes; 0 = not specified"`
	CookMinutes *int            `json:"cookMinutes,omitempty" jsonschema:"cook time in minutes; 0 = not specified"`
	Calories    *int            `json:"calories,omitempty" jsonschema:"kcal per serving; 0 = not specified"`
	SourceURL   *string         `json:"sourceUrl,omitempty" jsonschema:"http(s) URL the recipe came from"`
	Notes       *string         `json:"notes,omitempty" jsonschema:"markdown notes, tips, substitutions"`
	Status      *string         `json:"status,omitempty" jsonschema:"draft or published"`
	Tags        []string        `json:"tags,omitempty" jsonschema:"lowercase tags; replaces the whole list"`
	Ingredients []recipes.Group `json:"ingredients,omitempty" jsonschema:"ingredient groups, each {name, items[]}; use name \"\" for an ungrouped list; replaces the whole list"`
	Steps       []recipes.Step  `json:"steps,omitempty" jsonschema:"ordered directions, each {text, substeps[]}; replaces the whole list"`
	ImageFocusX *float64        `json:"imageFocusX,omitempty" jsonschema:"update only: photo focal point, 0 (left) to 1 (right); default 0.5"`
	ImageFocusY *float64        `json:"imageFocusY,omitempty" jsonschema:"update only: photo focal point, 0 (top) to 1 (bottom); default 0.5"`
	ImageZoom   *float64        `json:"imageZoom,omitempty" jsonschema:"update only: photo zoom around the focal point, 1 (whole photo) to 4"`
}

func (f recipeFields) toFields() recipes.Fields {
	out := recipes.Fields{
		Slug: f.Slug, Title: f.Title, Tagline: f.Tagline, Author: f.Author,
		Servings: f.Servings, PrepMinutes: f.PrepMinutes, CookMinutes: f.CookMinutes,
		Calories: f.Calories, SourceURL: f.SourceURL, Notes: f.Notes, Status: f.Status,
		ImageFocusX: f.ImageFocusX, ImageFocusY: f.ImageFocusY, ImageZoom: f.ImageZoom,
	}
	if f.Tags != nil {
		out.Tags = &f.Tags
	}
	if f.Ingredients != nil {
		out.Ingredients = &f.Ingredients
	}
	if f.Steps != nil {
		out.Steps = &f.Steps
	}
	return out
}

func (s *toolServer) createRecipe(ctx context.Context, _ *mcp.CallToolRequest, a recipeFields) (*mcp.CallToolResult, recipes.Recipe, error) {
	if !auth.CanWrite(ctx) {
		return errNoWrite, recipes.Recipe{}, nil
	}
	r, err := s.svc.Create(ctx, a.toFields())
	if err != nil {
		return fail[recipes.Recipe](err)
	}
	return nil, r, nil
}

type updateArgs struct {
	Recipe string `json:"recipe" jsonschema:"recipe id (uuid) or slug"`
	recipeFields
}

func (s *toolServer) updateRecipe(ctx context.Context, _ *mcp.CallToolRequest, a updateArgs) (*mcp.CallToolResult, recipes.Recipe, error) {
	if !auth.CanWrite(ctx) {
		return errNoWrite, recipes.Recipe{}, nil
	}
	r, err := s.svc.Update(ctx, a.Recipe, a.toFields())
	if err != nil {
		return fail[recipes.Recipe](err)
	}
	return nil, r, nil
}

type addStepArgs struct {
	Recipe   string   `json:"recipe" jsonschema:"recipe id (uuid) or slug"`
	Text     string   `json:"text" jsonschema:"the step text"`
	Substeps []string `json:"substeps,omitempty" jsonschema:"optional sub-steps"`
	Position int      `json:"position,omitempty" jsonschema:"1-based position to insert at; omit or 0 to append"`
}

func (s *toolServer) addStep(ctx context.Context, _ *mcp.CallToolRequest, a addStepArgs) (*mcp.CallToolResult, recipes.Recipe, error) {
	if !auth.CanWrite(ctx) {
		return errNoWrite, recipes.Recipe{}, nil
	}
	r, err := s.svc.AddStep(ctx, a.Recipe, recipes.Step{Text: a.Text, Substeps: a.Substeps}, a.Position)
	if err != nil {
		return fail[recipes.Recipe](err)
	}
	return nil, r, nil
}

type addIngredientArgs struct {
	Recipe string `json:"recipe" jsonschema:"recipe id (uuid) or slug"`
	Group  string `json:"group,omitempty" jsonschema:"ingredient group name, e.g. Broth; omit for the ungrouped list"`
	Text   string `json:"text" jsonschema:"the ingredient line, e.g. 2 cups chicken stock"`
}

func (s *toolServer) addIngredient(ctx context.Context, _ *mcp.CallToolRequest, a addIngredientArgs) (*mcp.CallToolResult, recipes.Recipe, error) {
	if !auth.CanWrite(ctx) {
		return errNoWrite, recipes.Recipe{}, nil
	}
	r, err := s.svc.AddIngredient(ctx, a.Recipe, a.Group, a.Text)
	if err != nil {
		return fail[recipes.Recipe](err)
	}
	return nil, r, nil
}

type deleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
	Note    string `json:"note"`
}

func (s *toolServer) deleteRecipe(ctx context.Context, _ *mcp.CallToolRequest, a refArgs) (*mcp.CallToolResult, deleteResult, error) {
	if !auth.CanWrite(ctx) {
		return errNoWrite, deleteResult{}, nil
	}
	r, err := s.svc.Delete(ctx, a.Recipe)
	if err != nil {
		return fail[deleteResult](err)
	}
	return nil, deleteResult{ID: r.ID, Deleted: true, Note: "restorable with restore_recipe_revision using this id"}, nil
}

type restoreArgs struct {
	Recipe   string `json:"recipe" jsonschema:"recipe id (uuid) or slug; use the id for a deleted recipe"`
	Revision int    `json:"revision" jsonschema:"revision number from get_recipe_history"`
}

func (s *toolServer) restoreRevision(ctx context.Context, _ *mcp.CallToolRequest, a restoreArgs) (*mcp.CallToolResult, recipes.Recipe, error) {
	if !auth.CanWrite(ctx) {
		return errNoWrite, recipes.Recipe{}, nil
	}
	r, err := s.svc.Restore(ctx, a.Recipe, a.Revision)
	if err != nil {
		return fail[recipes.Recipe](err)
	}
	return nil, r, nil
}

type createShareArgs struct {
	Recipe        string `json:"recipe" jsonschema:"recipe id (uuid) or slug"`
	ExpiresInDays int    `json:"expiresInDays,omitempty" jsonschema:"days until the link stops working; omit or 0 for no expiry"`
}

func (s *toolServer) createShare(ctx context.Context, _ *mcp.CallToolRequest, a createShareArgs) (*mcp.CallToolResult, recipes.Share, error) {
	if !auth.CanWrite(ctx) {
		return errNoWrite, recipes.Share{}, nil
	}
	sh, err := s.svc.CreateShare(ctx, a.Recipe, a.ExpiresInDays)
	if err != nil {
		return fail[recipes.Share](err)
	}
	return nil, sh, nil
}

func (s *toolServer) listShares(ctx context.Context, _ *mcp.CallToolRequest, a refArgs) (*mcp.CallToolResult, []recipes.Share, error) {
	shares, err := s.svc.ListShares(ctx, a.Recipe)
	if err != nil {
		return fail[[]recipes.Share](err)
	}
	return nil, shares, nil
}

type revokeShareArgs struct {
	Recipe  string `json:"recipe" jsonschema:"recipe id (uuid) or slug"`
	ShareID string `json:"shareId" jsonschema:"share id from list_share_links"`
}

type okResult struct {
	Revoked bool `json:"revoked"`
}

func (s *toolServer) revokeShare(ctx context.Context, _ *mcp.CallToolRequest, a revokeShareArgs) (*mcp.CallToolResult, okResult, error) {
	if !auth.CanWrite(ctx) {
		return errNoWrite, okResult{}, nil
	}
	if err := s.svc.RevokeShare(ctx, a.Recipe, a.ShareID); err != nil {
		return fail[okResult](err)
	}
	return nil, okResult{Revoked: true}, nil
}

type setImageArgs struct {
	Recipe      string `json:"recipe" jsonschema:"recipe id (uuid) or slug"`
	ImageURL    string `json:"imageUrl,omitempty" jsonschema:"public http(s) URL of a JPEG, PNG or WebP image"`
	ImageBase64 string `json:"imageBase64,omitempty" jsonschema:"the image bytes, base64-encoded (optionally as a data: URL)"`
	FocusX      *float64 `json:"focusX,omitempty" jsonschema:"optional focal point, 0 (left) to 1 (right); default 0.5 (centred)"`
	FocusY      *float64 `json:"focusY,omitempty" jsonschema:"optional focal point, 0 (top) to 1 (bottom); default 0.5 (centred)"`
	Zoom        *float64 `json:"zoom,omitempty" jsonschema:"optional zoom around the focal point, 1 (whole photo) to 4; default 1"`
}

type setImageResult struct {
	Recipe string `json:"recipe"`
	Set    bool   `json:"set"`
}

func (s *toolServer) setImage(ctx context.Context, _ *mcp.CallToolRequest, a setImageArgs) (*mcp.CallToolResult, setImageResult, error) {
	if !auth.CanWrite(ctx) {
		return errNoWrite, setImageResult{}, nil
	}
	if (a.ImageURL == "") == (a.ImageBase64 == "") {
		return errorResult("give exactly one of imageUrl or imageBase64"), setImageResult{}, nil
	}
	var err error
	if a.ImageURL != "" {
		err = s.svc.SetImageFromURL(ctx, a.Recipe, a.ImageURL)
	} else {
		raw := a.ImageBase64
		if i := strings.Index(raw, ","); strings.HasPrefix(raw, "data:") && i > 0 {
			raw = raw[i+1:]
		}
		var data []byte
		if data, err = base64.StdEncoding.DecodeString(strings.TrimSpace(raw)); err != nil {
			return errorResult("imageBase64 is not valid base64"), setImageResult{}, nil
		}
		err = s.svc.SetImage(ctx, a.Recipe, data)
	}
	if err != nil {
		return fail[setImageResult](err)
	}
	if a.FocusX != nil || a.FocusY != nil || a.Zoom != nil {
		if _, err := s.svc.Update(ctx, a.Recipe, recipes.Fields{ImageFocusX: a.FocusX, ImageFocusY: a.FocusY, ImageZoom: a.Zoom}); err != nil {
			return fail[setImageResult](err)
		}
	}
	return nil, setImageResult{Recipe: a.Recipe, Set: true}, nil
}
