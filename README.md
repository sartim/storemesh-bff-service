# StoreMesh BFF

The BFF is the client-facing Go edge for StoreMesh. It exposes REST/JSON for
resource and operational routes and GraphQL for composed client views. Both
surfaces use the domain services' generated gRPC clients internally; browsers
and mobile clients never call internal gRPC services directly.

## Local run

Requires Go 1.26.6 or newer. Start the domain services as local processes with
unique gRPC ports, or point these addresses at targeted port-forwards for
dependencies still running in Kind:

```sh
PRODUCT_SERVICE_ADDR=localhost:50051 \
ORDER_SERVICE_ADDR=localhost:50052 \
USER_SERVICE_ADDR=localhost:50053 \
go run ./cmd/server
```

The BFF listens on `HTTP_ADDR` (default `:8080`). For a different local port,
set `HTTP_ADDR=:8081`. The frontend should call this BFF address; it should
not call the internal gRPC services directly.

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
- `GET /api/v1/config`

GraphQL is available at `POST /api/v1/graphql` for authenticated client
composition. The live schema supports `products`, `cart`, and `orders` reads,
plus `updateCart`, `clearCart`, and idempotent `createOrder` mutations.

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

The GraphQL endpoint and schema are maintained alongside the existing REST
routes, keeping current clients compatible while allowing each client to
request only the fields needed by a screen. The BFF CI contract test verifies
the required commerce query and mutation fields remain available.

The versioned composition contract is maintained at
`api/graphql/schema.graphqls`. The live read resolvers are `products`, `cart`,
and `orders`. Cart and order queries are scoped to the authenticated token
subject and support the same pagination/status filters as the REST API.
GraphQL mutations now include `updateCart` and `createOrder`. Both enforce the
authenticated customer subject; `createOrder` requires a caller-provided
idempotency key. Web and mobile clients may consume GraphQL for these composed
views while continuing to use REST for login, health, uploads, and simple
resource mutations.

## Keycloak validation

## Runtime feature flags

The BFF is the server-side feature-flag boundary. Set `FLAGSMITH_API_KEY` and,
for self-hosted Flagsmith, `FLAGSMITH_BASE_URL` to enable OpenFeature
evaluation through the Flagsmith provider. If the key is absent or a flag
cannot be evaluated, typed safe defaults are used. `GET /api/v1/config` exposes
only client-safe evaluated flags to web and mobile clients; it is not an
authorization mechanism.

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
