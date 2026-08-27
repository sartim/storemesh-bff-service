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
- `GET /api/v1/orders/{id}`
- `POST /api/v1/orders/{id}:cancel`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`
- `GET /api/v1/users/{id}`

Incoming `Authorization` is forwarded as gRPC metadata. Login and refresh are
delegated to User Service; role-aware edge policy will be added as the
frontend journeys are implemented.

The BFF allows the configured `CORS_ALLOWED_ORIGIN` (default
`http://localhost:3000`) and maps gRPC failures to consistent JSON errors.

Build the image from the StoreMesh workspace root so local service module
replacements are available:

```sh
docker build -f storemesh-bff/Dockerfile .
```
