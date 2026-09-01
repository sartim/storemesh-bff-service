package main

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/graphql-go/graphql"
	productv1 "github.com/sartim/storemesh-product-service/gen/storemesh/product/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type graphQLRequest struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName"`
	Variables     map[string]interface{} `json:"variables"`
}

type graphQLServerKey struct{}

var productGraphQLSchema = buildProductGraphQLSchema()

func buildProductGraphQLSchema() graphql.Schema {
	productStatusType := graphql.NewEnum(graphql.EnumConfig{
		Name: "ProductStatus",
		Values: graphql.EnumValueConfigMap{
			"UNSPECIFIED": {Value: "UNSPECIFIED"},
			"ACTIVE":      {Value: "ACTIVE"},
			"ARCHIVED":    {Value: "ARCHIVED"},
		},
	})
	productType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Product",
		Fields: graphql.Fields{
			"id":          {Type: graphql.NewNonNull(graphql.String)},
			"sku":         {Type: graphql.NewNonNull(graphql.String)},
			"name":        {Type: graphql.NewNonNull(graphql.String)},
			"description": {Type: graphql.String},
			"priceMinor":  {Type: graphql.NewNonNull(graphql.Int)},
			"currency":    {Type: graphql.NewNonNull(graphql.String)},
			"status":      {Type: productStatusType},
		},
	})
	connectionType := graphql.NewObject(graphql.ObjectConfig{
		Name: "ProductConnection",
		Fields: graphql.Fields{
			"products":      {Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(productType)))},
			"nextPageToken": {Type: graphql.String},
		},
	})
	root := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"products": {
				Type: connectionType,
				Args: graphql.FieldConfigArgument{
					"pageSize":  {Type: graphql.Int},
					"pageToken": {Type: graphql.String},
					"status":    {Type: productStatusType},
				},
				Resolve: resolveProducts,
			},
		},
	})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: root})
	if err != nil {
		panic(err)
	}
	return schema
}

func resolveProducts(params graphql.ResolveParams) (interface{}, error) {
	s, ok := params.Context.Value(graphQLServerKey{}).(*server)
	if !ok || s == nil {
		return nil, status.Error(codes.Internal, "graphql server context is missing")
	}
	request := &productv1.ListProductsRequest{PageToken: stringArgument(params.Args, "pageToken"), Status: productStatus(stringArgument(params.Args, "status"))}
	if pageSize, ok := params.Args["pageSize"].(int); ok {
		request.PageSize = int32(pageSize)
	}
	response, err := s.products.ListProducts(params.Context, request)
	if err != nil {
		return nil, err
	}
	products := make([]map[string]interface{}, 0, len(response.GetProducts()))
	for _, product := range response.GetProducts() {
		products = append(products, map[string]interface{}{
			"id": product.GetId(), "sku": product.GetSku(), "name": product.GetName(),
			"description": product.GetDescription(), "priceMinor": product.GetPriceMinor(),
			"currency": product.GetCurrency(), "status": graphProductStatus(product.GetStatus()),
		})
	}
	return map[string]interface{}{"products": products, "nextPageToken": response.GetNextPageToken()}, nil
}

func graphProductStatus(status productv1.ProductStatus) string {
	switch status {
	case productv1.ProductStatus_PRODUCT_STATUS_ACTIVE:
		return "ACTIVE"
	case productv1.ProductStatus_PRODUCT_STATUS_ARCHIVED:
		return "ARCHIVED"
	default:
		return "UNSPECIFIED"
	}
}

func stringArgument(args map[string]interface{}, name string) string {
	value, _ := args[name].(string)
	return value
}

func (s *server) graphQL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	request := graphQLRequest{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Query == "" {
		http.Error(w, "invalid graphql request", http.StatusBadRequest)
		return
	}
	result := graphql.Do(graphql.Params{
		Schema:         productGraphQLSchema,
		RequestString:  request.Query,
		OperationName:  request.OperationName,
		VariableValues: request.Variables,
		Context:        context.WithValue(r.Context(), graphQLServerKey{}, s),
	})
	w.Header().Set("Content-Type", "application/json")
	if len(result.Errors) > 0 {
		w.WriteHeader(http.StatusBadGateway)
	}
	_ = json.NewEncoder(w).Encode(result)
}
