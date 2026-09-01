# StoreMesh BFF

The BFF is the client-facing Go edge for StoreMesh. It exposes REST/JSON for
resource and operational routes and GraphQL for composed client views. Both
surfaces use the domain services' generated gRPC clients internally; browsers
and mobile clients never call internal gRPC services directly.

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
local no-database mode. Strict Keycloak validation applies when the OIDC
configuration is enabled.

The BFF allows the configured `CORS_ALLOWED_ORIGIN` (default
`http://localhost:3000`) and maps gRPC failures to consistent JSON errors.

## API shape

REST remains the resource-oriented surface for authentication, health,
catalog, carts, orders, and administration. GraphQL is the composition
surface for views that combine multiple domains, such as an order summary with
product, inventory, and customer information. GraphQL resolvers must call the
generated gRPC clients through application services; they must not duplicate
domain rules or access another service's database.

The GraphQL endpoint and schema will be introduced incrementally alongside
the existing REST routes, keeping current clients compatible while allowing
each client to request only the fields needed by a screen.

The versioned composition contract is maintained at
`api/graphql/schema.graphqls`. Its initial scope is read-only product, cart,
and customer-order views. Mutations continue through REST until GraphQL
mutation authorization, idempotency, and error contracts are covered by
resolver tests. Web and mobile clients may consume GraphQL for composed views;
they should continue using REST for login, health, uploads, and simple
resource mutations.

## Keycloak validation

Set both `KEYCLOAK_ISSUER` and `KEYCLOAK_AUDIENCE` to enable strict OIDC
validation. The BFF discovers the realm JWKS endpoint, validates RS256
signatures, issuer, audience, expiry, and a small clock-skew allowance. If
these settings are omitted, local compatibility mode retains the existing
development token handling; shared environments must set them and should not
run in compatibility mode.

Strict validation protects orders, carts, user, and admin routes. `OPTIONS`
preflight requests remain public for browser CORS negotiation.

Build the image from the StoreMesh workspace root so local service module
replacements are available:

```sh
docker build -f storemesh-bff/Dockerfile .
```
