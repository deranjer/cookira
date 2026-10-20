package api

import (
	"context"
	"net/http"
	"strings"

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

type loginInput struct {
	Body struct {
		Email    string `json:"email" example:"you@example.com"`
		Password string `json:"password"`
	}
}

type loginOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
	Body      userBody
}

type logoutInput struct {
	Session string `cookie:"cookira_session"`
}

type logoutOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
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
	}, func(ctx context.Context, input *loginInput) (*loginOutput, error) {
		user, err := s.q.GetUserByEmail(ctx, strings.TrimSpace(input.Body.Email))
		if err != nil || !user.PasswordHash.Valid || !auth.CheckPassword(user.PasswordHash.String, input.Body.Password) {
			return nil, huma.Error401Unauthorized("invalid email or password")
		}
		cookie, err := s.mgr.CreateSession(ctx, user.ID)
		if err != nil {
			return nil, err
		}
		return &loginOutput{SetCookie: cookie, Body: userFromRow(user)}, nil
	})

	huma.Register(s.humaAPI, huma.Operation{
		OperationID: "logout",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/logout",
		Summary:     "Log out",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, input *logoutInput) (*logoutOutput, error) {
		_ = s.mgr.DeleteSession(ctx, input.Session)
		return &logoutOutput{SetCookie: auth.ClearedCookie(), Body: statusBody{Status: "ok"}}, nil
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
	}
}

type setupOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie"`
	Body      userBody
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
	}, func(ctx context.Context, input *setupInput) (*setupOutput, error) {
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
		cookie, err := s.mgr.CreateSession(ctx, user.ID)
		if err != nil {
			return nil, err
		}
		return &setupOutput{SetCookie: cookie, Body: userFromRow(user)}, nil
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
		Summary:     "Create a household member (admin only, session only)",
		Tags:        []string{"Users"},
		Middlewares: s.protected(),
	}, func(ctx context.Context, input *createUserInput) (*userOutput, error) {
		if auth.ViaAPIKey(ctx) {
			return nil, huma.Error403Forbidden("manage users from a signed-in session")
		}
		me, err := s.currentUser(ctx)
		if err != nil {
			return nil, err
		}
		if me.Role != "admin" {
			return nil, huma.Error403Forbidden("admin only")
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
}
