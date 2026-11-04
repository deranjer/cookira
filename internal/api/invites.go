package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/deranjer/cookira/internal/auth"
	"github.com/deranjer/cookira/internal/db"
)

const defaultInviteDays = 7

type inviteBody struct {
	ID        string  `json:"id"`
	Email     *string `json:"email,omitempty"`
	Role      string  `json:"role"`
	CreatedAt string  `json:"createdAt"`
	ExpiresAt string  `json:"expiresAt"`
	Expired   bool    `json:"expired"`
	// Link is only present in the response that creates the invite: the
	// token is stored hashed, so it can't be shown again.
	Link string `json:"link,omitempty"`
}

func inviteFromRow(i db.Invite) inviteBody {
	b := inviteBody{
		ID:        i.ID.String(),
		Role:      i.Role,
		CreatedAt: ts(i.CreatedAt),
		ExpiresAt: ts(i.ExpiresAt),
		Expired:   i.ExpiresAt.Time.Before(time.Now()),
	}
	if i.Email.Valid {
		b.Email = &i.Email.String
	}
	return b
}

type createInviteInput struct {
	Body struct {
		Email         string `json:"email,omitempty" doc:"Optional: lock the invite to this address."`
		Role          string `json:"role,omitempty" enum:"admin,member"`
		ExpiresInDays int    `json:"expiresInDays,omitempty" minimum:"0" maximum:"30" doc:"Defaults to 7."`
	}
}

type inviteOutput struct {
	Body inviteBody
}

type listInvitesOutput struct {
	Body []inviteBody
}

type inviteIDInput struct {
	ID string `path:"id"`
}

type inviteTokenInput struct {
	Token string `path:"token"`
}

type inviteInfoOutput struct {
	Body struct {
		Email *string `json:"email,omitempty"`
		Role  string  `json:"role"`
	}
}

type acceptInviteInput struct {
	Token string `path:"token"`
	Body  struct {
		Name     string `json:"name" minLength:"1"`
		Email    string `json:"email,omitempty" doc:"Required unless the invite is locked to an address."`
		Password string `json:"password" minLength:"8"`
		Tokens   bool   `json:"tokens,omitempty"`
	}
}

func pgTextOpt(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

func pgTimestamp(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func (s *Server) inviteLink(token string) string {
	return strings.TrimRight(s.publicURL, "/") + "/invite/" + token
}

func (s *Server) registerInviteRoutes() {
	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "create-invite",
		Method:      http.MethodPost,
		Path:        "/api/v1/invites",
		Summary:     "Create an invite link (admin only)",
		Description: "The link is returned once. The invitee opens it and chooses their own name and password.",
		Tags:        []string{"Invites"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, input *createInviteInput) (*inviteOutput, error) {
		me, err := s.requireAdmin(ctx)
		if err != nil {
			return nil, err
		}
		role := input.Body.Role
		if role == "" {
			role = "member"
		}
		days := input.Body.ExpiresInDays
		if days == 0 {
			days = defaultInviteDays
		}
		email := strings.TrimSpace(input.Body.Email)
		if email != "" {
			if !strings.Contains(email, "@") {
				return nil, huma.Error422UnprocessableEntity("invalid email")
			}
			if _, err := s.q.GetUserByEmail(ctx, email); err == nil {
				return nil, huma.Error409Conflict("a user with that email already exists")
			}
		}
		token, hash := auth.GenerateInviteToken()
		inv, err := s.q.CreateInvite(ctx, db.CreateInviteParams{
			TokenHash: hash,
			Email:     pgTextOpt(email),
			Role:      role,
			CreatedBy: me.ID,
			ExpiresAt: pgTimestamp(time.Now().Add(time.Duration(days) * 24 * time.Hour)),
		})
		if err != nil {
			return nil, err
		}
		body := inviteFromRow(inv)
		body.Link = s.inviteLink(token)
		return &inviteOutput{Body: body}, nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "list-invites",
		Method:      http.MethodGet,
		Path:        "/api/v1/invites",
		Summary:     "List unused invites (admin only)",
		Tags:        []string{"Invites"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, input *struct{}) (*listInvitesOutput, error) {
		if _, err := s.requireAdmin(ctx); err != nil {
			return nil, err
		}
		rows, err := s.q.ListInvites(ctx)
		if err != nil {
			return nil, err
		}
		out := &listInvitesOutput{Body: make([]inviteBody, len(rows))}
		for i, r := range rows {
			out.Body[i] = inviteFromRow(r)
		}
		return out, nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "revoke-invite",
		Method:      http.MethodDelete,
		Path:        "/api/v1/invites/{id}",
		Summary:     "Revoke an unused invite (admin only)",
		Tags:        []string{"Invites"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, input *inviteIDInput) (*statusOutput, error) {
		if _, err := s.requireAdmin(ctx); err != nil {
			return nil, err
		}
		id, err := mustUUID(input.ID)
		if err != nil {
			return nil, huma.Error404NotFound("invite not found")
		}
		n, err := s.q.DeleteInvite(ctx, id)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return nil, huma.Error404NotFound("invite not found")
		}
		return okOutput(), nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "get-invite",
		Method:      http.MethodGet,
		Path:        "/api/v1/invites/lookup/{token}",
		Summary:     "Check an invite link before accepting it (public)",
		Tags:        []string{"Invites"},
	}, func(ctx context.Context, input *inviteTokenInput) (*inviteInfoOutput, error) {
		inv, err := s.q.GetInviteByHash(ctx, auth.HashToken(input.Token))
		if err != nil || inv.UsedAt.Valid || inv.ExpiresAt.Time.Before(time.Now()) {
			return nil, huma.Error404NotFound("this invite link is invalid or has expired")
		}
		out := &inviteInfoOutput{}
		out.Body.Role = inv.Role
		if inv.Email.Valid {
			out.Body.Email = &inv.Email.String
		}
		return out, nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "accept-invite",
		Method:      http.MethodPost,
		Path:        "/api/v1/invites/accept/{token}",
		Summary:     "Accept an invite: create your account and sign in (public)",
		Tags:        []string{"Invites"},
	}, func(ctx context.Context, input *acceptInviteInput) (*authOutput, error) {
		user, err := s.acceptInvite(ctx, input)
		if err != nil {
			return nil, err
		}
		return s.signIn(ctx, user, input.Body.Tokens)
	})
}

// acceptInvite redeems the invite and creates the user in one transaction,
// so a failure (e.g. the email is taken) leaves the invite usable.
func (s *Server) acceptInvite(ctx context.Context, input *acceptInviteInput) (db.User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	inv, err := q.ClaimInvite(ctx, auth.HashToken(input.Token))
	if errors.Is(err, pgx.ErrNoRows) {
		return db.User{}, huma.Error404NotFound("this invite link is invalid or has expired")
	}
	if err != nil {
		return db.User{}, err
	}

	email := strings.TrimSpace(input.Body.Email)
	if inv.Email.Valid {
		if email != "" && !strings.EqualFold(email, inv.Email.String) {
			return db.User{}, huma.Error422UnprocessableEntity("this invite is for a different email address")
		}
		email = inv.Email.String
	}
	if !strings.Contains(email, "@") {
		return db.User{}, huma.Error422UnprocessableEntity("a valid email is required")
	}
	if _, err := q.GetUserByEmail(ctx, email); err == nil {
		return db.User{}, huma.Error409Conflict("a user with that email already exists")
	}

	hash, err := auth.HashPassword(input.Body.Password)
	if err != nil {
		return db.User{}, err
	}
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Name: strings.TrimSpace(input.Body.Name), Email: email,
		PasswordHash: pgText(hash), Role: inv.Role,
	})
	if err != nil {
		return db.User{}, err
	}
	if err := q.SetInviteUsedBy(ctx, db.SetInviteUsedByParams{ID: inv.ID, UsedBy: user.ID}); err != nil {
		return db.User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.User{}, err
	}
	return user, nil
}
