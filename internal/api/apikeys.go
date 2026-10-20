package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/deranjer/cookira/internal/auth"
	"github.com/deranjer/cookira/internal/db"
)

type apiKeyBody struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Scopes     []string `json:"scopes"`
	CreatedAt  string   `json:"createdAt"`
	LastUsedAt *string  `json:"lastUsedAt"`
}

func apiKeyToBody(k db.ApiKey) apiKeyBody {
	return apiKeyBody{
		ID:         k.ID.String(),
		Name:       k.Name,
		Scopes:     k.Scopes,
		CreatedAt:  ts(k.CreatedAt),
		LastUsedAt: nullableTimestamp(k.LastUsedAt),
	}
}

type listAPIKeysOutput struct {
	Body []apiKeyBody
}

type createAPIKeyInput struct {
	Body struct {
		Name     string `json:"name" minLength:"1"`
		ReadOnly bool   `json:"readOnly"`
	}
}

type createAPIKeyOutput struct {
	Body struct {
		ID         string   `json:"id"`
		Name       string   `json:"name"`
		Scopes     []string `json:"scopes"`
		CreatedAt  string   `json:"createdAt"`
		LastUsedAt *string  `json:"lastUsedAt"`
		Key        string   `json:"key"` // plaintext, shown once
	}
}

type deleteAPIKeyInput struct {
	ID string `path:"id"`
}

type deleteAPIKeyOutput struct {
	Body struct {
		Status string `json:"status"`
	}
}

func (s *Server) registerAPIKeyRoutes() {
	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "list-api-keys",
		Method:      http.MethodGet,
		Path:        "/api/v1/api-keys",
		Summary:     "List the current user's API keys",
		Tags:        []string{"API Keys"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, input *struct{}) (*listAPIKeysOutput, error) {
		userIDStr, _ := auth.UserID(ctx)
		userID, err := mustUUID(userIDStr)
		if err != nil {
			return nil, huma.Error401Unauthorized("login required")
		}
		keys, err := s.q.ListAPIKeysByUser(ctx, userID)
		if err != nil {
			return nil, err
		}
		out := &listAPIKeysOutput{Body: make([]apiKeyBody, len(keys))}
		for i, k := range keys {
			out.Body[i] = apiKeyToBody(k)
		}
		return out, nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "create-api-key",
		Method:      http.MethodPost,
		Path:        "/api/v1/api-keys",
		Summary:     "Create an API key (plaintext shown once)",
		Tags:        []string{"API Keys"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, input *createAPIKeyInput) (*createAPIKeyOutput, error) {
		// API-key management is session-only: a leaked read-only key must
		// never be usable to mint itself a write key (or any new key).
		if auth.ViaAPIKey(ctx) {
			return nil, huma.Error403Forbidden("manage API keys from a signed-in session")
		}
		userIDStr, _ := auth.UserID(ctx)
		userID, err := mustUUID(userIDStr)
		if err != nil {
			return nil, huma.Error401Unauthorized("login required")
		}
		plaintext, hash := auth.GenerateAPIKey()
		created, err := s.q.CreateAPIKey(ctx, db.CreateAPIKeyParams{
			UserID:  userID,
			Name:    input.Body.Name,
			KeyHash: hash,
			Scopes:  auth.ScopesForWrite(!input.Body.ReadOnly),
		})
		if err != nil {
			return nil, err
		}
		body := apiKeyToBody(created)
		out := &createAPIKeyOutput{}
		out.Body.ID = body.ID
		out.Body.Name = body.Name
		out.Body.Scopes = body.Scopes
		out.Body.CreatedAt = body.CreatedAt
		out.Body.LastUsedAt = body.LastUsedAt
		out.Body.Key = plaintext
		return out, nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "delete-api-key",
		Method:      http.MethodDelete,
		Path:        "/api/v1/api-keys/{id}",
		Summary:     "Revoke an API key",
		Tags:        []string{"API Keys"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, input *deleteAPIKeyInput) (*deleteAPIKeyOutput, error) {
		if auth.ViaAPIKey(ctx) {
			return nil, huma.Error403Forbidden("manage API keys from a signed-in session")
		}
		userIDStr, _ := auth.UserID(ctx)
		userID, err := mustUUID(userIDStr)
		if err != nil {
			return nil, huma.Error401Unauthorized("login required")
		}
		id, err := mustUUID(input.ID)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid id")
		}
		rows, err := s.q.DeleteAPIKey(ctx, db.DeleteAPIKeyParams{ID: id, UserID: userID})
		if err != nil {
			return nil, err
		}
		if rows == 0 {
			return nil, huma.Error404NotFound("api key not found")
		}
		out := &deleteAPIKeyOutput{}
		out.Body.Status = "ok"
		return out, nil
	})
}
