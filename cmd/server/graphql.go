package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/graphql-go/graphql"
	productv1 "github.com/sartim/storemesh-product-service/gen/storemesh/product/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	orderv1 "storemesh-order-service/gen/storemesh/order/v1"
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
	productStatusArgument := productStatusType
	orderStatusType := graphql.NewEnum(graphql.EnumConfig{
		Name: "OrderStatus",
		Values: graphql.EnumValueConfigMap{
			"UNSPECIFIED": {Value: "UNSPECIFIED"}, "PENDING": {Value: "PENDING"},
			"CONFIRMED": {Value: "CONFIRMED"}, "CANCELLED": {Value: "CANCELLED"},
		},
	})
	cartLineType := graphql.NewObject(graphql.ObjectConfig{Name: "CartLine", Fields: graphql.Fields{
		"productId": {Type: graphql.NewNonNull(graphql.String)}, "quantity": {Type: graphql.NewNonNull(graphql.Int)},
	}})
	cartType := graphql.NewObject(graphql.ObjectConfig{Name: "Cart", Fields: graphql.Fields{
		"customerId": {Type: graphql.NewNonNull(graphql.String)}, "lines": {Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(cartLineType)))},
	}})
	orderType := graphql.NewObject(graphql.ObjectConfig{Name: "Order", Fields: graphql.Fields{
		"id": {Type: graphql.NewNonNull(graphql.String)}, "customerId": {Type: graphql.NewNonNull(graphql.String)},
		"status": {Type: graphql.NewNonNull(orderStatusType)}, "totalMinor": {Type: graphql.NewNonNull(graphql.Int)},
		"currency": {Type: graphql.NewNonNull(graphql.String)}, "createdAt": {Type: graphql.NewNonNull(graphql.String)},
	}})
	orderConnectionType := graphql.NewObject(graphql.ObjectConfig{Name: "OrderConnection", Fields: graphql.Fields{
		"orders": {Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(orderType)))}, "nextPageToken": {Type: graphql.String},
	}})
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
					"status":    {Type: productStatusArgument},
				},
				Resolve: resolveProducts,
			},
			"cart": {Type: cartType, Resolve: resolveCart},
			"orders": {Type: orderConnectionType, Args: graphql.FieldConfigArgument{
				"pageSize": {Type: graphql.Int}, "pageToken": {Type: graphql.String}, "status": {Type: orderStatusType},
			}, Resolve: resolveOrders},
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

func resolveCart(params graphql.ResolveParams) (interface{}, error) {
	s, customerID, err := graphQLAuth(params)
	if err != nil {
		return nil, err
	}
	response, err := s.carts.GetCart(params.Context, &orderv1.GetCartRequest{CustomerId: customerID})
	if err != nil {
		return nil, err
	}
	lines := make([]map[string]interface{}, 0, len(response.GetCart().GetLines()))
	for _, line := range response.GetCart().GetLines() {
		lines = append(lines, map[string]interface{}{"productId": line.GetProductId(), "quantity": line.GetQuantity()})
	}
	return map[string]interface{}{"customerId": customerID, "lines": lines}, nil
}

func resolveOrders(params graphql.ResolveParams) (interface{}, error) {
	s, customerID, err := graphQLAuth(params)
	if err != nil {
		return nil, err
	}
	request := &orderv1.ListOrdersRequest{CustomerId: customerID, PageToken: stringArgument(params.Args, "pageToken"), Status: orderStatus(stringArgument(params.Args, "status"))}
	if pageSize, ok := params.Args["pageSize"].(int); ok {
		request.PageSize = int32(pageSize)
	}
	response, err := s.orders.ListOrders(params.Context, request)
	if err != nil {
		return nil, err
	}
	orders := make([]map[string]interface{}, 0, len(response.GetOrders()))
	for _, order := range response.GetOrders() {
		createdAt := ""
		if order.GetCreatedAt() != nil {
			createdAt = order.GetCreatedAt().AsTime().Format(time.RFC3339)
		}
		orders = append(orders, map[string]interface{}{"id": order.GetOrderId(), "customerId": order.GetCustomerId(), "status": graphOrderStatus(order.GetStatus()), "totalMinor": order.GetTotalMinor(), "currency": order.GetCurrency(), "createdAt": createdAt})
	}
	return map[string]interface{}{"orders": orders, "nextPageToken": response.GetNextPageToken()}, nil
}

func graphQLAuth(params graphql.ResolveParams) (*server, string, error) {
	s, ok := params.Context.Value(graphQLServerKey{}).(*server)
	if !ok || s == nil {
		return nil, "", status.Error(codes.Internal, "graphql server context is missing")
	}
	customerID, _ := params.Context.Value(graphQLCustomerKey{}).(string)
	if customerID == "" {
		return nil, "", status.Error(codes.Unauthenticated, "customer subject is required")
	}
	return s, customerID, nil
}

type graphQLCustomerKey struct{}

func graphOrderStatus(status orderv1.OrderStatus) string {
	switch status {
	case orderv1.OrderStatus_ORDER_STATUS_PENDING:
		return "PENDING"
	case orderv1.OrderStatus_ORDER_STATUS_CONFIRMED:
		return "CONFIRMED"
	case orderv1.OrderStatus_ORDER_STATUS_CANCELLED:
		return "CANCELLED"
	default:
		return "UNSPECIFIED"
	}
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
		Context:        context.WithValue(context.WithValue(grpcContext(r), graphQLServerKey{}, s), graphQLCustomerKey{}, s.customerSubject(r.Header.Get("Authorization"))),
	})
	w.Header().Set("Content-Type", "application/json")
	if len(result.Errors) > 0 {
		w.WriteHeader(http.StatusBadGateway)
	}
	_ = json.NewEncoder(w).Encode(result)
}
