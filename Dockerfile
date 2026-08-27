FROM golang:1.26.6 AS build
WORKDIR /src/storemesh-bff
COPY storemesh-bff/go.mod storemesh-bff/go.sum ./
COPY storemesh-bff/cmd ./cmd
COPY storemesh-order-service /src/storemesh-order-service
COPY storemesh-product-service /src/storemesh-product-service
COPY storemesh-inventory-service /src/storemesh-inventory-service
COPY storemesh-user-service /src/storemesh-user-service
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/storemesh-bff ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/storemesh-bff /storemesh-bff
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/storemesh-bff"]
