package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/deranjer/cookira/internal/auth"
	"github.com/deranjer/cookira/internal/db"
)

type userBody struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

func userFromRow(u db.User) userBody {
	return userBody{ID: u.ID.String(), Name: u.Name, Email: u.Email, Role: u.Role}
}

// tokenBody carries credentials in the response body for clients that
// don't use cookies (mobile apps): request it with `tokens: true` on login,
// setup or invite acceptance, and send the refresh token in the body of
// /auth/refresh. Access tokens go in an `Authorization: Bearer` header.
type tokenBody struct {
	AccessToken      string `json:"accessToken"`
	AccessExpiresAt  string `json:"accessExpiresAt"`
	RefreshToken     string `json:"refreshToken"`
	RefreshExpiresAt string `json:"refreshExpiresAt"`
}

// sessionBody is the user plus, for token-mode clients, their credentials.
type sessionBody struct {
	ID     string     `json:"id"`
	Name   string     `json:"name"`
	Email  string     `json:"email"`
	Role   string     `json:"role"`
	Tokens *tokenBody `json:"tokens,omitempty"`
}

// authOutput is the response to anything that signs a user in. Browser
// clients get httpOnly cookies; token-mode clients get Tokens in the body
// and no cookies.
type authOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Body      sessionBody
}

// signIn issues credentials for user and shapes them for the client kind.
func (s *Server) signIn(ctx context.Context, user db.User, wantTokens bool) (*authOutput, error) {
	t, err := s.mgr.IssueTokens(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	out := &authOutput{Body: sessionBody{ID: user.ID.String(), Name: user.Name, Email: user.Email, Role: user.Role}}
	if wantTokens {
		out.Body.Tokens = &tokenBody{
			AccessToken:      t.Access,
			AccessExpiresAt:  t.AccessExp.Format(time.RFC3339),
			RefreshToken:     t.Refresh,
			RefreshExpiresAt: t.RefreshExp.Format(time.RFC3339),
		}
		return out, nil
	}
	out.SetCookie = []http.Cookie{auth.AccessCookie(t.Access, t.AccessExp), auth.RefreshCookie(t.Refresh, t.RefreshExp)}
	return out, nil
}

type loginInput struct {
	Body struct {
		Email    string `json:"email" example:"you@example.com"`
		Password string `json:"password"`
		Tokens   bool   `json:"tokens,omitempty" doc:"Return tokens in the body instead of setting cookies (mobile clients)."`
	}
}

type refreshInput struct {
	RefreshCookie string `cookie:"cookira_refresh"`
	Body          *struct {
		RefreshToken string `json:"refreshToken,omitempty" doc:"Token-mode clients send their refresh token here; browsers rely on the cookie."`
	}
}

// token returns the refresh token and whether the client is in token mode.
func (in *refreshInput) token() (string, bool) {
	if in.Body != nil && in.Body.RefreshToken != "" {
		return in.Body.RefreshToken, true
	}
	return in.RefreshCookie, false
}

type logoutOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Body      statusBody
}

type userOutput struct {
	Body userBody
}

func (s *Server) registerAuthRoutes() {
	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "login",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/login",
		Summary:     "Log in with email and password",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, input *loginInput) (*authOutput, error) {
		email := strings.TrimSpace(input.Body.Email)
		if !s.limiter.Allowed(email) {
			return nil, huma.Error429TooManyRequests("too many failed attempts; try again in a few minutes")
		}
		user, err := s.q.GetUserByEmail(ctx, email)
		if err != nil || !user.PasswordHash.Valid || !auth.CheckPassword(user.PasswordHash.String, input.Body.Password) {
			s.limiter.Fail(email)
			return nil, huma.Error401Unauthorized("invalid email or password")
		}
		s.limiter.Reset(email)
		return s.signIn(ctx, user, input.Body.Tokens)
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "refresh",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/refresh",
		Summary:     "Exchange a refresh token for a new access/refresh pair",
		Description: "Refresh tokens are single-use and rotate on every call. Replaying an already-used token revokes all of that user's refresh tokens.",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, input *refreshInput) (*authOutput, error) {
		raw, tokenMode := input.token()
		userID, t, err := s.mgr.Refresh(ctx, raw)
		if err != nil {
			return nil, huma.Error401Unauthorized("invalid or expired refresh token")
		}
		user, err := s.q.GetUserByID(ctx, userID)
		if err != nil {
			return nil, huma.Error401Unauthorized("invalid or expired refresh token")
		}
		out := &authOutput{Body: sessionBody{ID: user.ID.String(), Name: user.Name, Email: user.Email, Role: user.Role}}
		if tokenMode {
			out.Body.Tokens = &tokenBody{
				AccessToken:      t.Access,
				AccessExpiresAt:  t.AccessExp.Format(time.RFC3339),
				RefreshToken:     t.Refresh,
				RefreshExpiresAt: t.RefreshExp.Format(time.RFC3339),
			}
			return out, nil
		}
		out.SetCookie = []http.Cookie{auth.AccessCookie(t.Access, t.AccessExp), auth.RefreshCookie(t.Refresh, t.RefreshExp)}
		return out, nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "logout",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/logout",
		Summary:     "Log out (revokes the refresh token)",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, input *refreshInput) (*logoutOutput, error) {
		raw, _ := input.token()
		s.mgr.Revoke(ctx, raw)
		return &logoutOutput{SetCookie: auth.ClearedCookies(), Body: statusBody{Status: "ok"}}, nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "me",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/me",
		Summary:     "Get the current authenticated user",
		Tags:        []string{"Auth"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, input *struct{}) (*userOutput, error) {
		user, err := s.currentUser(ctx)
		if err != nil {
			return nil, err
		}
		return &userOutput{Body: userFromRow(user)}, nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "update-me",
		Method:      http.MethodPatch,
		Path:        "/api/v1/auth/me",
		Summary:     "Update your name or password",
		Description: "Changing the password signs out every other device. Send the current refresh token (cookie or body) to keep this device signed in.",
		Tags:        []string{"Auth"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, input *updateMeInput) (*userOutput, error) {
		user, err := s.currentUser(ctx)
		if err != nil {
			return nil, err
		}
		name := user.Name
		if n := input.Body.Name; n != nil {
			if name = strings.TrimSpace(*n); name == "" {
				return nil, huma.Error422UnprocessableEntity("name cannot be empty")
			}
		}
		if p := input.Body.Password; p != nil {
			if !user.PasswordHash.Valid || !auth.CheckPassword(user.PasswordHash.String, input.Body.CurrentPassword) {
				return nil, huma.Error403Forbidden("current password is incorrect")
			}
			hash, err := auth.HashPassword(*p)
			if err != nil {
				return nil, err
			}
			if err := s.q.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{ID: user.ID, PasswordHash: pgText(hash)}); err != nil {
				return nil, err
			}
			keep := input.RefreshCookie
			if input.Body.RefreshToken != "" {
				keep = input.Body.RefreshToken
			}
			if err := s.mgr.RevokeAll(ctx, user.ID, keep); err != nil {
				return nil, err
			}
		}
		if name != user.Name {
			if user, err = s.q.UpdateUserProfile(ctx, db.UpdateUserProfileParams{ID: user.ID, Name: name, Role: user.Role}); err != nil {
				return nil, err
			}
		}
		return &userOutput{Body: userFromRow(user)}, nil
	})
}

type updateMeInput struct {
	RefreshCookie string `cookie:"cookira_refresh"`
	Body          struct {
		Name            *string `json:"name,omitempty"`
		CurrentPassword string  `json:"currentPassword,omitempty"`
		Password        *string `json:"password,omitempty" minLength:"8"`
		RefreshToken    string  `json:"refreshToken,omitempty" doc:"Token-mode clients: the refresh token to keep valid."`
	}
}

func (s *Server) currentUser(ctx context.Context) (db.User, error) {
	idStr, _ := auth.UserID(ctx)
	id, err := mustUUID(idStr)
	if err != nil {
		return db.User{}, huma.Error401Unauthorized("login required")
	}
	user, err := s.q.GetUserByID(ctx, id)
	if err != nil {
		return db.User{}, huma.Error401Unauthorized("login required")
	}
	return user, nil
}

// requireAdmin returns the caller if they are an admin.
func (s *Server) requireAdmin(ctx context.Context) (db.User, error) {
	me, err := s.currentUser(ctx)
	if err != nil {
		return db.User{}, err
	}
	if me.Role != "admin" {
		return db.User{}, huma.Error403Forbidden("admin only")
	}
	return me, nil
}

// ---- first-run setup ----

type setupStatusOutput struct {
	Body struct {
		Required bool `json:"required"`
	}
}

type setupInput struct {
	Body struct {
		Name     string `json:"name" minLength:"1"`
		Email    string `json:"email" example:"you@example.com"`
		Password string `json:"password" minLength:"8"`
		Tokens   bool   `json:"tokens,omitempty"`
	}
}

// registerSetupRoutes wires the first-run wizard: once any user exists,
// db.SetupRequired permanently reports false, so /setup can never run again.
func (s *Server) registerSetupRoutes() {
	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "setup-status",
		Method:      http.MethodGet,
		Path:        "/api/v1/setup/status",
		Summary:     "Check whether first-run setup is still required",
		Tags:        []string{"Setup"},
	}, func(ctx context.Context, input *struct{}) (*setupStatusOutput, error) {
		required, err := db.SetupRequired(ctx, s.q)
		if err != nil {
			return nil, err
		}
		out := &setupStatusOutput{}
		out.Body.Required = required
		return out, nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "setup-complete",
		Method:      http.MethodPost,
		Path:        "/api/v1/setup",
		Summary:     "Create the admin user and finish first-run setup",
		Tags:        []string{"Setup"},
	}, func(ctx context.Context, input *setupInput) (*authOutput, error) {
		required, err := db.SetupRequired(ctx, s.q)
		if err != nil {
			return nil, err
		}
		if !required {
			return nil, huma.Error409Conflict("setup has already been completed")
		}
		user, err := db.CompleteSetup(ctx, s.q, strings.TrimSpace(input.Body.Name), strings.TrimSpace(input.Body.Email), input.Body.Password)
		if err != nil {
			return nil, err
		}
		return s.signIn(ctx, user, input.Body.Tokens)
	})
}

// ---- users ----

type listUsersOutput struct {
	Body []userBody
}

type createUserInput struct {
	Body struct {
		Name     string `json:"name" minLength:"1"`
		Email    string `json:"email"`
		Password string `json:"password" minLength:"8"`
		Role     string `json:"role,omitempty" enum:"admin,member"`
	}
}

type updateUserInput struct {
	ID   string `path:"id"`
	Body struct {
		Name     *string `json:"name,omitempty"`
		Role     *string `json:"role,omitempty" enum:"admin,member"`
		Password *string `json:"password,omitempty" minLength:"8" doc:"Admin password reset; signs the user out everywhere."`
	}
}

type userIDInput struct {
	ID string `path:"id"`
}

func (s *Server) registerUserRoutes() {
	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "list-users",
		Method:      http.MethodGet,
		Path:        "/api/v1/users",
		Summary:     "List household members",
		Tags:        []string{"Users"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, input *struct{}) (*listUsersOutput, error) {
		users, err := s.q.ListUsers(ctx)
		if err != nil {
			return nil, err
		}
		out := &listUsersOutput{Body: make([]userBody, len(users))}
		for i, u := range users {
			out.Body[i] = userFromRow(u)
		}
		return out, nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "create-user",
		Method:      http.MethodPost,
		Path:        "/api/v1/users",
		Summary:     "Create a household member with a password (admin only, session only)",
		Description: "Prefer invite links (POST /invites); this is for when you want to set the password yourself.",
		Tags:        []string{"Users"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, input *createUserInput) (*userOutput, error) {
		if _, err := s.requireAdmin(ctx); err != nil {
			return nil, err
		}
		hash, err := auth.HashPassword(input.Body.Password)
		if err != nil {
			return nil, err
		}
		role := input.Body.Role
		if role == "" {
			role = "member"
		}
		email := strings.TrimSpace(input.Body.Email)
		if !strings.Contains(email, "@") {
			return nil, huma.Error422UnprocessableEntity("invalid email")
		}
		if _, err := s.q.GetUserByEmail(ctx, email); err == nil {
			return nil, huma.Error409Conflict("a user with that email already exists")
		}
		user, err := s.q.CreateUser(ctx, db.CreateUserParams{
			Name: strings.TrimSpace(input.Body.Name), Email: email,
			PasswordHash: pgText(hash), Role: role,
		})
		if err != nil {
			return nil, err
		}
		return &userOutput{Body: userFromRow(user)}, nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "update-user",
		Method:      http.MethodPatch,
		Path:        "/api/v1/users/{id}",
		Summary:     "Change a member's name or role, or reset their password (admin only)",
		Tags:        []string{"Users"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, input *updateUserInput) (*userOutput, error) {
		if _, err := s.requireAdmin(ctx); err != nil {
			return nil, err
		}
		target, err := s.userByIDParam(ctx, input.ID)
		if err != nil {
			return nil, err
		}
		name, role := target.Name, target.Role
		if n := input.Body.Name; n != nil {
			if name = strings.TrimSpace(*n); name == "" {
				return nil, huma.Error422UnprocessableEntity("name cannot be empty")
			}
		}
		if r := input.Body.Role; r != nil {
			role = *r
		}
		if target.Role == "admin" && role != "admin" {
			if err := s.ensureOtherAdmin(ctx); err != nil {
				return nil, err
			}
		}
		updated, err := s.q.UpdateUserProfile(ctx, db.UpdateUserProfileParams{ID: target.ID, Name: name, Role: role})
		if err != nil {
			return nil, err
		}
		if p := input.Body.Password; p != nil {
			hash, err := auth.HashPassword(*p)
			if err != nil {
				return nil, err
			}
			if err := s.q.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{ID: target.ID, PasswordHash: pgText(hash)}); err != nil {
				return nil, err
			}
			if err := s.mgr.RevokeAll(ctx, target.ID, ""); err != nil {
				return nil, err
			}
		}
		return &userOutput{Body: userFromRow(updated)}, nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "delete-user",
		Method:      http.MethodDelete,
		Path:        "/api/v1/users/{id}",
		Summary:     "Remove a member (admin only)",
		Tags:        []string{"Users"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, input *userIDInput) (*statusOutput, error) {
		me, err := s.requireAdmin(ctx)
		if err != nil {
			return nil, err
		}
		target, err := s.userByIDParam(ctx, input.ID)
		if err != nil {
			return nil, err
		}
		if target.ID == me.ID {
			return nil, huma.Error409Conflict("you can't remove your own account")
		}
		if err := s.q.DeleteUser(ctx, target.ID); err != nil {
			return nil, err
		}
		return okOutput(), nil
	})
}

func (s *Server) userByIDParam(ctx context.Context, id string) (db.User, error) {
	uid, err := mustUUID(id)
	if err != nil {
		return db.User{}, huma.Error404NotFound("user not found")
	}
	u, err := s.q.GetUserByID(ctx, uid)
	if err != nil {
		return db.User{}, huma.Error404NotFound("user not found")
	}
	return u, nil
}

// ensureOtherAdmin refuses to demote the last remaining admin.
func (s *Server) ensureOtherAdmin(ctx context.Context) error {
	n, err := s.q.CountAdmins(ctx)
	if err != nil {
		return err
	}
	if n <= 1 {
		return huma.Error409Conflict("there must be at least one admin")
	}
	return nil
}
