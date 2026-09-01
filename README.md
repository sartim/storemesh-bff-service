# StoreMesh BFF

The BFF is the client-facing REST edge for StoreMesh. It keeps the public
route namespace stable while using the domain services' generated gRPC clients
internally.

## Local run

```sh
PRODUCT_SERVICE_ADDR=localhost:50051 \
ORDER_SERVICE_ADDR=localhost:50052 \
USER_SERVICE_ADDR=localhost:50053 \
go run ./cmd/server
```

Defaults target the Kubernetes Service DNS names. The initial milestone
includes:

- `GET /healthz`
- `GET /api/v1/products`
- `GET /api/v1/products/{id}`
- `POST /api/v1/orders`
- `GET /api/v1/orders?customer_id={id}&page_size={n}&page_token={token}&status={status}`
- `GET /api/v1/orders/{id}`
- `POST /api/v1/orders/{id}:cancel`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`
- `GET /api/v1/users/{id}`
- `GET /api/v1/admin/users`
- `DELETE /api/v1/admin/users/{id}`
- `GET /api/v1/admin/users/{id}/roles`
- `PUT /api/v1/admin/users/{id}/roles/{role}`
- `DELETE /api/v1/admin/users/{id}/roles/{role}`
- `GET /api/v1/admin/roles`
- `GET /api/v1/cart?customer_id={id}`
- `PUT /api/v1/cart?customer_id={id}`
- `DELETE /api/v1/cart?customer_id={id}`

Incoming `Authorization` is forwarded as gRPC metadata. Login and refresh are
delegated to User Service. Admin routes require an Authorization header at the
edge and User Service performs the final token and admin-role authorization.
Order listing supports customer and status filters with page-token pagination;
the caller is responsible for selecting an authorized customer scope until
the order authorization interceptor is introduced.
Cart routes require the supplied customer ID to match the bearer token `sub`
claim and persist through the Order Service cart store. The Order Service
uses PostgreSQL when `DATABASE_URL` is configured and in-memory storage for
local no-database mode. Full JWT signature enforcement remains part of the
shared authorization interceptor work.

The BFF allows the configured `CORS_ALLOWED_ORIGIN` (default
`http://localhost:3000`) and maps gRPC failures to consistent JSON errors.

Build the image from the StoreMesh workspace root so local service module
replacements are available:

```sh
docker build -f storemesh-bff/Dockerfile .
```
