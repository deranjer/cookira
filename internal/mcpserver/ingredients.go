package mcpserver

import (
	"context"
	"github.com/deranjer/cookira/internal/auth"
	"github.com/deranjer/cookira/internal/recipes"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *toolServer) listIngredients(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, []recipes.CatalogIngredient, error) {
	items, err := s.svc.Catalog(ctx)
	if err != nil {
		return fail[[]recipes.CatalogIngredient](err)
	}
	return nil, items, nil
}

func (s *toolServer) saveIngredient(ctx context.Context, _ *mcp.CallToolRequest, in recipes.CatalogIngredient) (*mcp.CallToolResult, recipes.CatalogIngredient, error) {
	if !auth.CanWrite(ctx) {
		return errNoWrite, recipes.CatalogIngredient{}, nil
	}
	item, err := s.svc.SaveCatalog(ctx, in)
	if err != nil {
		return fail[recipes.CatalogIngredient](err)
	}
	return nil, item, nil
}
