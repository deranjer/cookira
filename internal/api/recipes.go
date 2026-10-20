package api

import (
	"context"
	"io"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/deranjer/cookira/internal/auth"
	"github.com/deranjer/cookira/internal/recipes"
)

func pgText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }

// recipeFieldsBody is the editable part of a recipe. On PATCH every field is
// optional and absent fields are left alone; on create, absent fields take
// defaults (status defaults to "draft").
type recipeFieldsBody struct {
	Slug        *string         `json:"slug,omitempty" doc:"URL slug; generated from the title when omitted"`
	Title       *string         `json:"title,omitempty"`
	Tagline     *string         `json:"tagline,omitempty"`
	Author      *string         `json:"author,omitempty"`
	Servings    *int            `json:"servings,omitempty" doc:"0 = not specified"`
	PrepMinutes *int            `json:"prepMinutes,omitempty"`
	CookMinutes *int            `json:"cookMinutes,omitempty"`
	Calories    *int            `json:"calories,omitempty"`
	SourceURL   *string         `json:"sourceUrl,omitempty"`
	Notes       *string         `json:"notes,omitempty" doc:"Markdown"`
	Status      *string         `json:"status,omitempty" enum:"draft,published"`
	Tags        []string        `json:"tags,omitempty"`
	Ingredients []recipes.Group `json:"ingredients,omitempty"`
	Steps       []recipes.Step  `json:"steps,omitempty"`
}

func (b recipeFieldsBody) fields() recipes.Fields {
	f := recipes.Fields{
		Slug: b.Slug, Title: b.Title, Tagline: b.Tagline, Author: b.Author,
		Servings: b.Servings, PrepMinutes: b.PrepMinutes, CookMinutes: b.CookMinutes,
		Calories: b.Calories, SourceURL: b.SourceURL, Notes: b.Notes, Status: b.Status,
	}
	if b.Tags != nil {
		f.Tags = &b.Tags
	}
	if b.Ingredients != nil {
		f.Ingredients = &b.Ingredients
	}
	if b.Steps != nil {
		f.Steps = &b.Steps
	}
	return f
}

type recipeOutput struct {
	Body recipes.Recipe
}

type refInput struct {
	Ref string `path:"ref" doc:"Recipe id (uuid) or slug"`
}

type listRecipesInput struct {
	Q      string `query:"q" doc:"Full-text search"`
	Tag    string `query:"tag"`
	Status string `query:"status" enum:"draft,published,"`
	Limit  int    `query:"limit" default:"50" maximum:"200"`
	Offset int    `query:"offset" default:"0"`
}

type listRecipesOutput struct {
	Body struct {
		Items []recipes.Summary `json:"items"`
		Total int               `json:"total"`
	}
}

type createRecipeInput struct {
	Body recipeFieldsBody
}

type updateRecipeInput struct {
	Ref  string `path:"ref"`
	Body recipeFieldsBody
}

type listTagsOutput struct {
	Body []recipes.TagCount
}

type listRevisionsOutput struct {
	Body []recipes.Revision
}

type restoreInput struct {
	Ref      string `path:"ref"`
	Revision int    `path:"revision"`
}

func (s *Server) registerRecipeRoutes() {
	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "list-recipes",
		Method:      http.MethodGet,
		Path:        "/api/v1/recipes",
		Summary:     "List and search recipes",
		Tags:        []string{"Recipes"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, in *listRecipesInput) (*listRecipesOutput, error) {
		items, total, err := s.svc.List(ctx, recipes.ListParams{
			Query: in.Q, Tag: in.Tag, Status: in.Status, Limit: in.Limit, Offset: in.Offset,
		})
		if err != nil {
			return nil, mapErr(err)
		}
		out := &listRecipesOutput{}
		out.Body.Items = items
		out.Body.Total = total
		return out, nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "list-tags",
		Method:      http.MethodGet,
		Path:        "/api/v1/tags",
		Summary:     "List tags with recipe counts",
		Tags:        []string{"Recipes"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, _ *struct{}) (*listTagsOutput, error) {
		tags, err := s.svc.Tags(ctx)
		if err != nil {
			return nil, mapErr(err)
		}
		return &listTagsOutput{Body: tags}, nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "create-recipe",
		Method:      http.MethodPost,
		Path:        "/api/v1/recipes",
		Summary:     "Create a recipe (draft unless status is given)",
		Tags:        []string{"Recipes"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, in *createRecipeInput) (*recipeOutput, error) {
		if err := requireWrite(ctx); err != nil {
			return nil, err
		}
		r, err := s.svc.Create(ctx, in.Body.fields())
		if err != nil {
			return nil, mapErr(err)
		}
		return &recipeOutput{Body: r}, nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "get-recipe",
		Method:      http.MethodGet,
		Path:        "/api/v1/recipes/{ref}",
		Summary:     "Get a recipe by id or slug",
		Tags:        []string{"Recipes"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, in *refInput) (*recipeOutput, error) {
		r, err := s.svc.Get(ctx, in.Ref)
		if err != nil {
			return nil, mapErr(err)
		}
		return &recipeOutput{Body: r}, nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "update-recipe",
		Method:      http.MethodPatch,
		Path:        "/api/v1/recipes/{ref}",
		Summary:     "Update a recipe (only supplied fields change)",
		Tags:        []string{"Recipes"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, in *updateRecipeInput) (*recipeOutput, error) {
		if err := requireWrite(ctx); err != nil {
			return nil, err
		}
		r, err := s.svc.Update(ctx, in.Ref, in.Body.fields())
		if err != nil {
			return nil, mapErr(err)
		}
		return &recipeOutput{Body: r}, nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "delete-recipe",
		Method:      http.MethodDelete,
		Path:        "/api/v1/recipes/{ref}",
		Summary:     "Delete a recipe (soft delete; restorable from its history)",
		Tags:        []string{"Recipes"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, in *refInput) (*statusOutput, error) {
		if err := requireWrite(ctx); err != nil {
			return nil, err
		}
		if _, err := s.svc.Delete(ctx, in.Ref); err != nil {
			return nil, mapErr(err)
		}
		return okOutput(), nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "list-recipe-revisions",
		Method:      http.MethodGet,
		Path:        "/api/v1/recipes/{ref}/revisions",
		Summary:     "List a recipe's change history",
		Tags:        []string{"Recipes"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, in *refInput) (*listRevisionsOutput, error) {
		revs, err := s.svc.Revisions(ctx, in.Ref)
		if err != nil {
			return nil, mapErr(err)
		}
		return &listRevisionsOutput{Body: revs}, nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "restore-recipe-revision",
		Method:      http.MethodPost,
		Path:        "/api/v1/recipes/{ref}/revisions/{revision}/restore",
		Summary:     "Restore a recipe's content from an earlier revision",
		Tags:        []string{"Recipes"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, in *restoreInput) (*recipeOutput, error) {
		if err := requireWrite(ctx); err != nil {
			return nil, err
		}
		r, err := s.svc.Restore(ctx, in.Ref, in.Revision)
		if err != nil {
			return nil, mapErr(err)
		}
		return &recipeOutput{Body: r}, nil
	})
}

// ---- share links ----

type createShareInput struct {
	Ref  string `path:"ref"`
	Body struct {
		ExpiresInDays int `json:"expiresInDays,omitempty" doc:"0 or omitted = never expires"`
	}
}

type shareOutput struct {
	Body recipes.Share
}

type listSharesOutput struct {
	Body []recipes.Share
}

type revokeShareInput struct {
	Ref     string `path:"ref"`
	ShareID string `path:"shareId"`
}

func (s *Server) registerShareRoutes() {
	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "create-share",
		Method:      http.MethodPost,
		Path:        "/api/v1/recipes/{ref}/shares",
		Summary:     "Create a view-only share link",
		Tags:        []string{"Sharing"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, in *createShareInput) (*shareOutput, error) {
		if err := requireWrite(ctx); err != nil {
			return nil, err
		}
		sh, err := s.svc.CreateShare(ctx, in.Ref, in.Body.ExpiresInDays)
		if err != nil {
			return nil, mapErr(err)
		}
		return &shareOutput{Body: sh}, nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "list-shares",
		Method:      http.MethodGet,
		Path:        "/api/v1/recipes/{ref}/shares",
		Summary:     "List a recipe's active share links",
		Tags:        []string{"Sharing"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, in *refInput) (*listSharesOutput, error) {
		shares, err := s.svc.ListShares(ctx, in.Ref)
		if err != nil {
			return nil, mapErr(err)
		}
		return &listSharesOutput{Body: shares}, nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "revoke-share",
		Method:      http.MethodDelete,
		Path:        "/api/v1/recipes/{ref}/shares/{shareId}",
		Summary:     "Revoke a share link",
		Tags:        []string{"Sharing"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, in *revokeShareInput) (*statusOutput, error) {
		if err := requireWrite(ctx); err != nil {
			return nil, err
		}
		if err := s.svc.RevokeShare(ctx, in.Ref, in.ShareID); err != nil {
			return nil, mapErr(err)
		}
		return okOutput(), nil
	})
}

// registerPublicRoutes serves the unauthenticated, token-gated, read-only
// view that share links point at. Nothing else is reachable without login.
func (s *Server) registerPublicRoutes() {
	type tokenInput struct {
		Token string `path:"token"`
	}
	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "get-shared-recipe",
		Method:      http.MethodGet,
		Path:        "/api/v1/public/{token}",
		Summary:     "Fetch a recipe through a share link (no login)",
		Tags:        []string{"Sharing"},
	}, func(ctx context.Context, in *tokenInput) (*recipeOutput, error) {
		r, err := s.svc.GetShared(ctx, in.Token)
		if err != nil {
			return nil, mapErr(err)
		}
		// The share view never reveals ids, slugs or internal timestamps
		// that would help someone guess at other recipes.
		r.ID, r.Slug = "", ""
		return &recipeOutput{Body: r}, nil
	})
}

// registerImageRoutes handles recipe header images with plain chi handlers
// (streaming bodies and multipart uploads don't fit huma operations).
// ?size=thumb on the GET routes returns the small list-card version.
func (s *Server) registerImageRoutes(r *chi.Mux) {
	r.With(auth.RequireAuth(s.mgr)).Get("/api/v1/recipes/{ref}/image", func(w http.ResponseWriter, req *http.Request) {
		f, ctype, err := s.svc.OpenImage(req.Context(), chi.URLParam(req, "ref"), req.URL.Query().Get("size") == "thumb")
		serveImage(w, f, ctype, err, "private, no-cache")
	})

	r.Get("/api/v1/public/{token}/image", func(w http.ResponseWriter, req *http.Request) {
		f, ctype, err := s.svc.OpenSharedImage(req.Context(), chi.URLParam(req, "token"), req.URL.Query().Get("size") == "thumb")
		serveImage(w, f, ctype, err, "private, max-age=300")
	})

	r.With(auth.RequireAuth(s.mgr)).Put("/api/v1/recipes/{ref}/image", func(w http.ResponseWriter, req *http.Request) {
		if !auth.CanWrite(req.Context()) {
			writeProblem(w, http.StatusForbidden, "this API key is read-only")
			return
		}
		req.Body = http.MaxBytesReader(w, req.Body, recipes.MaxImageBytes+1<<20)
		file, _, err := req.FormFile("file")
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "expected a multipart form with a \"file\" field no larger than 10 MB")
			return
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, recipes.MaxImageBytes+1))
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "could not read upload")
			return
		}
		if err := s.svc.SetImage(req.Context(), chi.URLParam(req, "ref"), data); err != nil {
			writeServiceError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})

	r.With(auth.RequireAuth(s.mgr)).Delete("/api/v1/recipes/{ref}/image", func(w http.ResponseWriter, req *http.Request) {
		if !auth.CanWrite(req.Context()) {
			writeProblem(w, http.StatusForbidden, "this API key is read-only")
			return
		}
		if err := s.svc.RemoveImage(req.Context(), chi.URLParam(req, "ref")); err != nil {
			writeServiceError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})
}

func writeServiceError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case isNotFound(err):
		status = http.StatusNotFound
	case isInvalid(err):
		status = http.StatusUnprocessableEntity
	}
	writeProblem(w, status, err.Error())
}

func serveImage(w http.ResponseWriter, f io.ReadCloser, ctype string, err error, cache string) {
	if err != nil {
		writeProblem(w, http.StatusNotFound, "not found")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Cache-Control", cache)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.Copy(w, f)
}

func writeProblem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, `{"status":`+strconv.Itoa(status)+`,"title":`+strconv.Quote(http.StatusText(status))+`,"detail":`+strconv.Quote(detail)+`}`)
}
