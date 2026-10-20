package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

// SetupRequired reports whether the app has never been set up: no users
// exist yet, so the /setup wizard should run. There is no signup flow, so
// this is the only gate on first-run state; once an admin exists it can
// never report true again.
func SetupRequired(ctx context.Context, q *Queries) (bool, error) {
	count, err := q.CountUsers(ctx)
	if err != nil {
		return false, fmt.Errorf("counting users: %w", err)
	}
	return count == 0, nil
}

// CompleteSetup creates the first (admin) user. Callers must check
// SetupRequired first.
func CompleteSetup(ctx context.Context, q *Queries, name, email, password string) (User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, fmt.Errorf("hashing password: %w", err)
	}
	user, err := q.CreateUser(ctx, CreateUserParams{
		Name:         name,
		Email:        email,
		PasswordHash: pgtype.Text{String: string(hash), Valid: true},
		Role:         "admin",
	})
	if err != nil {
		return User{}, fmt.Errorf("creating admin user: %w", err)
	}
	return user, nil
}
