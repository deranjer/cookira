package api

import (
	"errors"

	"github.com/deranjer/cookira/internal/recipes"
)

func isNotFound(err error) bool { return errors.Is(err, recipes.ErrNotFound) }
func isInvalid(err error) bool  { return errors.Is(err, recipes.ErrInvalid) }
