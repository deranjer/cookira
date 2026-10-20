package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/deranjer/cookira/internal/auth"
	"github.com/deranjer/cookira/internal/db"
)

type oauthConnectionBody struct {
	ID         string   `json:"id"`
	ClientName string   `json:"clientName"`
	Scopes     []string `json:"scopes"`
	CreatedAt  string   `json:"createdAt"`
	LastUsedAt *string  `json:"lastUsedAt"`
}

type listOAuthConnectionsOutput struct {
	Body []oauthConnectionBody
}

type deleteOAuthConnectionInput struct {
	ID string `path:"id"`
}

// registerOAuthConnectionRoutes lists and revokes the apps (ChatGPT, ...)
// that were connected through the OAuth consent screen.
func (s *Server) registerOAuthConnectionRoutes() {
	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "list-oauth-connections",
		Method:      http.MethodGet,
		Path:        "/api/v1/oauth/connections",
		Summary:     "List apps connected through OAuth",
		Tags:        []string{"OAuth"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, input *struct{}) (*listOAuthConnectionsOutput, error) {
		userIDStr, _ := auth.UserID(ctx)
		userID, err := mustUUID(userIDStr)
		if err != nil {
			return nil, huma.Error401Unauthorized("login required")
		}
		rows, err := s.q.ListOAuthGrantsByUser(ctx, userID)
		if err != nil {
			return nil, err
		}
		out := &listOAuthConnectionsOutput{Body: make([]oauthConnectionBody, len(rows))}
		for i, g := range rows {
			out.Body[i] = oauthConnectionBody{
				ID:         g.ID.String(),
				ClientName: g.ClientName,
				Scopes:     g.Scopes,
				CreatedAt:  ts(g.CreatedAt),
				LastUsedAt: nullableTimestamp(g.LastUsedAt),
			}
		}
		return out, nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "delete-oauth-connection",
		Method:      http.MethodDelete,
		Path:        "/api/v1/oauth/connections/{id}",
		Summary:     "Disconnect an app (revokes its tokens)",
		Tags:        []string{"OAuth"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, input *deleteOAuthConnectionInput) (*statusOutput, error) {
		if auth.ViaAPIKey(ctx) {
			return nil, huma.Error403Forbidden("manage connections from a signed-in session")
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
		rows, err := s.q.DeleteOAuthGrant(ctx, db.DeleteOAuthGrantParams{ID: id, UserID: userID})
		if err != nil {
			return nil, err
		}
		if rows == 0 {
			return nil, huma.Error404NotFound("connection not found")
		}
		return okOutput(), nil
	})
}
