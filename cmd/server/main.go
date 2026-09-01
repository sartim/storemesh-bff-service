package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	productv1 "github.com/sartim/storemesh-product-service/gen/storemesh/product/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	orderv1 "storemesh-order-service/gen/storemesh/order/v1"
	userv1 "storemesh-user-service/gen/user/v1"
)

type server struct {
	products productv1.ProductCatalogServiceClient
	orders   orderv1.OrderServiceClient
	carts    orderv1.CartServiceClient
	users    userv1.UserServiceClient
}

func main() {
	productConn := dial(env("PRODUCT_SERVICE_ADDR", "storemesh-product-service:50051"))
	defer productConn.Close()
	orderConn := dial(env("ORDER_SERVICE_ADDR", "storemesh-order-service:50051"))
	defer orderConn.Close()
	userConn := dial(env("USER_SERVICE_ADDR", "storemesh-user-service:50051"))
	defer userConn.Close()

	s := &server{
		products: productv1.NewProductCatalogServiceClient(productConn),
		orders:   orderv1.NewOrderServiceClient(orderConn),
		carts:    orderv1.NewCartServiceClient(orderConn),
		users:    userv1.NewUserServiceClient(userConn),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/api/v1/products", s.productsRoute)
	mux.HandleFunc("/api/v1/orders", s.ordersRoute)
	mux.HandleFunc("/api/v1/cart", s.cartRoute)
	mux.HandleFunc("/api/v1/auth/login", s.login)
	mux.HandleFunc("/api/v1/auth/refresh", s.refresh)
	mux.HandleFunc("/api/v1/admin/", s.adminRoute)
	mux.HandleFunc("/api/v1/users/", s.userRoute)
	addr := env("HTTP_ADDR", ":8080")
	log.Printf("storemesh BFF listening on %s", addr)
	server := &http.Server{Addr: addr, Handler: cors(mux), ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(server.ListenAndServe())
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	request := &userv1.AuthenticateRequest{}
	if err := decode(r, request); err != nil {
		http.Error(w, "invalid credentials", http.StatusBadRequest)
		return
	}
	response, err := s.users.Authenticate(r.Context(), request)
	s.write(w, response, err)
}

func (s *server) refresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	request := &userv1.RefreshTokenRequest{}
	if err := decode(r, request); err != nil {
		http.Error(w, "invalid refresh token", http.StatusBadRequest)
		return
	}
	response, err := s.users.RefreshToken(r.Context(), request)
	s.write(w, response, err)
}

func (s *server) userRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := pathID(r.URL.Path, "/api/v1/users/")
	if id == "" {
		http.Error(w, "user ID is required", http.StatusBadRequest)
		return
	}
	response, err := s.users.GetUser(grpcContext(r), &userv1.GetUserRequest{Id: id})
	s.write(w, response, err)
}

// adminRoute exposes the User Service management APIs under an explicit BFF
// namespace. User Service remains the authorization authority and receives
// the original bearer token through grpcContext.
func (s *server) adminRoute(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") == "" {
		writeError(w, status.Error(codes.Unauthenticated, "authorization is required"))
		return
	}

	ctx := grpcContext(r)
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/")
	if path == "products" || strings.HasPrefix(path, "products/") {
		if r.Method == http.MethodGet && path == "products" {
			response, err := s.products.ListProducts(ctx, &productv1.ListProductsRequest{PageSize: 100})
			s.write(w, response, err)
			return
		}
		if r.Method == http.MethodPost && path == "products" {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "invalid product", http.StatusBadRequest)
				return
			}
			product := &productv1.Product{}
			if err := protojson.Unmarshal(body, product); err != nil {
				http.Error(w, "invalid product", http.StatusBadRequest)
				return
			}
			response, err := s.products.CreateProduct(ctx, &productv1.CreateProductRequest{Product: product})
			s.write(w, response, err)
			return
		}
		if r.Method == http.MethodDelete && strings.HasPrefix(path, "products/") {
			response, err := s.products.ArchiveProduct(ctx, &productv1.ArchiveProductRequest{Id: strings.TrimPrefix(path, "products/")})
			s.write(w, response, err)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if path == "roles" {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		response, err := s.users.ListRoles(ctx, &userv1.ListRolesRequest{})
		s.write(w, response, err)
		return
	}

	if !strings.HasPrefix(path, "users") {
		http.NotFound(w, r)
		return
	}
	path = strings.TrimPrefix(path, "users")
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		request := &userv1.ListUsersRequest{Status: r.URL.Query().Get("status")}
		if value := r.URL.Query().Get("page"); value != "" {
			if _, err := fmt.Sscan(value, &request.Page); err != nil {
				writeError(w, status.Error(codes.InvalidArgument, "page must be an integer"))
				return
			}
		}
		if value := r.URL.Query().Get("per_page"); value != "" {
			if _, err := fmt.Sscan(value, &request.PerPage); err != nil {
				writeError(w, status.Error(codes.InvalidArgument, "per_page must be an integer"))
				return
			}
		}
		response, err := s.users.ListUsers(ctx, request)
		s.write(w, response, err)
		return
	}

	parts := strings.Split(path, "/")
	userID := parts[0]
	if userID == "" {
		http.Error(w, "user ID is required", http.StatusBadRequest)
		return
	}
	if len(parts) == 1 {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		response, err := s.users.DeleteUser(ctx, &userv1.DeleteUserRequest{Id: userID})
		s.write(w, response, err)
		return
	}
	if parts[1] != "roles" {
		http.NotFound(w, r)
		return
	}
	if len(parts) == 2 {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		response, err := s.users.GetUserRoles(ctx, &userv1.GetUserRolesRequest{UserId: userID})
		s.write(w, response, err)
		return
	}
	if len(parts) != 3 || parts[2] == "" {
		http.NotFound(w, r)
		return
	}
	request := &userv1.AssignRoleRequest{UserId: userID, Role: parts[2]}
	if r.Method == http.MethodPut {
		response, err := s.users.AssignRole(ctx, request)
		s.write(w, response, err)
		return
	}
	if r.Method == http.MethodDelete {
		response, err := s.users.RevokeRole(ctx, &userv1.RevokeRoleRequest{UserId: userID, Role: parts[2]})
		s.write(w, response, err)
		return
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *server) productsRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	ctx := grpcContext(r)
	switch r.Method {
	case http.MethodGet:
		if id := pathID(r.URL.Path, "/api/v1/products/"); id != "" {
			response, err := s.products.GetProduct(ctx, &productv1.GetProductRequest{Id: id})
			s.write(w, response, err)
			return
		}
		request := &productv1.ListProductsRequest{PageToken: r.URL.Query().Get("page_token"), Status: productStatus(r.URL.Query().Get("status"))}
		if value := r.URL.Query().Get("page_size"); value != "" {
			if _, err := fmt.Sscan(value, &request.PageSize); err != nil {
				writeError(w, status.Error(codes.InvalidArgument, "page_size must be an integer"))
				return
			}
		}
		response, err := s.products.ListProducts(ctx, request)
		s.write(w, response, err)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) ordersRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	ctx := grpcContext(r)
	id := pathID(r.URL.Path, "/api/v1/orders/")
	switch {
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, ":cancel"):
		response, err := s.orders.CancelOrder(ctx, &orderv1.CancelOrderRequest{OrderId: strings.TrimSuffix(id, ":cancel")})
		s.write(w, response, err)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/orders":
		request := &orderv1.ListOrdersRequest{CustomerId: r.URL.Query().Get("customer_id"), PageToken: r.URL.Query().Get("page_token"), Status: orderStatus(r.URL.Query().Get("status"))}
		if value := r.URL.Query().Get("page_size"); value != "" {
			if _, err := fmt.Sscan(value, &request.PageSize); err != nil {
				writeError(w, status.Error(codes.InvalidArgument, "page_size must be an integer"))
				return
			}
		}
		response, err := s.orders.ListOrders(ctx, request)
		s.write(w, response, err)
	case r.Method == http.MethodGet && id != "":
		response, err := s.orders.GetOrder(ctx, &orderv1.GetOrderRequest{OrderId: id})
		s.write(w, response, err)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/orders":
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		order := &orderv1.Order{}
		if err := protojson.Unmarshal(body, order); err != nil {
			http.Error(w, "invalid order", http.StatusBadRequest)
			return
		}
		request := &orderv1.CreateOrderRequest{Order: order, IdempotencyKey: r.Header.Get("Idempotency-Key")}
		response, err := s.orders.CreateOrder(ctx, request)
		s.write(w, response, err)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) cartRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Header.Get("Authorization") == "" {
		writeError(w, status.Error(codes.Unauthenticated, "authorization is required"))
		return
	}
	customerID := r.URL.Query().Get("customer_id")
	if customerID == "" {
		writeError(w, status.Error(codes.InvalidArgument, "customer_id is required"))
		return
	}
	if subject := bearerSubject(r.Header.Get("Authorization")); subject == "" || subject != customerID {
		writeError(w, status.Error(codes.PermissionDenied, "customer_id must match the authenticated user"))
		return
	}
	ctx := grpcContext(r)
	switch r.Method {
	case http.MethodGet:
		response, err := s.carts.GetCart(ctx, &orderv1.GetCartRequest{CustomerId: customerID})
		s.write(w, response, err)
	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "invalid cart", http.StatusBadRequest)
			return
		}
		cart := &orderv1.Cart{}
		if err := protojson.Unmarshal(body, cart); err != nil {
			http.Error(w, "invalid cart", http.StatusBadRequest)
			return
		}
		cart.CustomerId = customerID
		response, err := s.carts.UpsertCart(ctx, &orderv1.UpsertCartRequest{Cart: cart})
		s.write(w, response, err)
	case http.MethodDelete:
		response, err := s.carts.ClearCart(ctx, &orderv1.ClearCartRequest{CustomerId: customerID})
		s.write(w, response, err)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func bearerSubject(header string) string {
	parts := strings.Split(strings.TrimPrefix(header, "Bearer "), ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Subject string `json:"sub"`
	}
	if json.Unmarshal(payload, &claims) != nil {
		return ""
	}
	return claims.Subject
}

func (s *server) write(w http.ResponseWriter, message proto.Message, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	if message == nil {
		writeError(w, status.Error(codes.Internal, "empty service response"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	payload, marshalErr := protojson.Marshal(message)
	if marshalErr != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(append(payload, '\n'))
}

func writeError(w http.ResponseWriter, err error) {
	code := status.Code(err)
	httpCode := map[codes.Code]int{
		codes.InvalidArgument:    http.StatusBadRequest,
		codes.Unauthenticated:    http.StatusUnauthorized,
		codes.PermissionDenied:   http.StatusForbidden,
		codes.NotFound:           http.StatusNotFound,
		codes.AlreadyExists:      http.StatusConflict,
		codes.FailedPrecondition: http.StatusPreconditionFailed,
		codes.Unavailable:        http.StatusServiceUnavailable,
		codes.DeadlineExceeded:   http.StatusGatewayTimeout,
	}[code]
	if httpCode == 0 {
		httpCode = http.StatusBadGateway
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)
	fmt.Fprintf(w, "{\"error\":{\"code\":%q,\"message\":%q}}\n", code.String(), status.Convert(err).Message())
}

func cors(next http.Handler) http.Handler {
	allowed := env("CORS_ALLOWED_ORIGIN", "http://localhost:3000")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func decode(r *http.Request, message proto.Message) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return protojson.Unmarshal(body, message)
}

func grpcContext(r *http.Request) context.Context {
	ctx := r.Context()
	if authorization := r.Header.Get("Authorization"); authorization != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", authorization)
	}
	return ctx
}

func dial(address string) *grpc.ClientConn {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial %s: %v", address, err)
	}
	return conn
}

func pathID(path, prefix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	return strings.TrimPrefix(path, prefix)
}

func productStatus(value string) productv1.ProductStatus {
	switch strings.ToLower(value) {
	case "active":
		return productv1.ProductStatus_PRODUCT_STATUS_ACTIVE
	case "archived":
		return productv1.ProductStatus_PRODUCT_STATUS_ARCHIVED
	default:
		return productv1.ProductStatus_PRODUCT_STATUS_UNSPECIFIED
	}
}

func orderStatus(value string) orderv1.OrderStatus {
	switch strings.ToLower(value) {
	case "pending":
		return orderv1.OrderStatus_ORDER_STATUS_PENDING
	case "confirmed":
		return orderv1.OrderStatus_ORDER_STATUS_CONFIRMED
	case "cancelled":
		return orderv1.OrderStatus_ORDER_STATUS_CANCELLED
	default:
		return orderv1.OrderStatus_ORDER_STATUS_UNSPECIFIED
	}
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
