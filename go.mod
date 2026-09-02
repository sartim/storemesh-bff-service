module storemesh-bff

go 1.26.6

require (
	github.com/Flagsmith/flagsmith-go-client/v5 v5.1.0
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/graphql-go/graphql v0.8.1
	github.com/open-feature/go-sdk v1.17.0
	github.com/open-feature/go-sdk-contrib/providers/flagsmith v0.1.6
	github.com/sartim/storemesh-product-service v0.0.0
	google.golang.org/grpc v1.83.1
	google.golang.org/protobuf v1.36.12
	storemesh-order-service v0.0.0
	storemesh-user-service v0.0.0
)

require (
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-resty/resty/v2 v2.17.2 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.30.0 // indirect
	github.com/itlightning/dateparse v0.2.1 // indirect
	github.com/ohler55/ojg v1.28.1 // indirect
	go.uber.org/mock v0.6.0 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260803160001-6ac0973c030d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260803160001-6ac0973c030d // indirect
)

replace storemesh-order-service => ../storemesh-order-service

replace storemesh-user-service => ../storemesh-user-service

replace github.com/sartim/storemesh-product-service => ../storemesh-product-service
