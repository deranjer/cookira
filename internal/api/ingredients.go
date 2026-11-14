package api

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/deranjer/cookira/internal/recipes"
	"net/http"
)

func registerCatalogRoutes(s *Server) {
	type catalogOutput struct{ Body []recipes.CatalogIngredient }
	type saveInput struct{ Body recipes.CatalogIngredient }
	type saveOutput struct{ Body recipes.CatalogIngredient }
	huma.Register(s.humaAPI, huma.Operation{OperationID: "list-ingredients", Method: http.MethodGet, Path: "/api/v1/ingredients", Summary: "List the shared ingredient catalog and aliases", Middlewares: s.protected()}, func(ctx context.Context, _ *struct{}) (*catalogOutput, error) {
		items, err := s.svc.Catalog(ctx)
		if err != nil {
			return nil, mapErr(err)
		}
		return &catalogOutput{Body: items}, nil
	})
	huma.Register(s.humaAPI, huma.Operation{OperationID: "save-ingredient", Method: http.MethodPost, Path: "/api/v1/ingredients", Summary: "Create or edit an ingredient (supply id to edit)", Middlewares: s.protected()}, func(ctx context.Context, in *saveInput) (*saveOutput, error) {
		item, err := s.svc.SaveCatalog(ctx, in.Body)
		if err != nil {
			return nil, mapErr(err)
		}
		return &saveOutput{Body: item}, nil
	})
}
