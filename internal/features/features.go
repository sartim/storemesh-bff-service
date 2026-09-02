package features

import (
	"context"
	"strings"

	flagsmithClient "github.com/Flagsmith/flagsmith-go-client/v5"
	flagsmithProvider "github.com/open-feature/go-sdk-contrib/providers/flagsmith/pkg"
	"github.com/open-feature/go-sdk/openfeature"
)

const domain = "storemesh-bff"

const (
	GraphQLCheckout = "graphql_checkout"
	KafkaAnalytics  = "kafka_analytics"
	AdminDashboard  = "admin_dashboard_v2"
	MobileCart      = "mobile_cart_v2"
)

// Flags evaluates runtime product flags. When Flagsmith is not configured,
// evaluations return the supplied defaults, keeping local development and
// platform bootstrap independent of the flag service.
type Flags struct {
	client  *openfeature.Client
	enabled bool
}

func New(apiKey, baseURL string) (*Flags, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return &Flags{}, nil
	}

	options := []flagsmithClient.Option{
		flagsmithClient.WithLocalEvaluation(context.Background()),
	}
	if strings.TrimSpace(baseURL) != "" {
		options = append(options, flagsmithClient.WithBaseURL(strings.TrimRight(baseURL, "/")+"/"))
	}

	provider := flagsmithProvider.NewProvider(
		flagsmithClient.NewClient(apiKey, options...),
		flagsmithProvider.WithUsingBooleanConfigValue(),
	)
	if err := openfeature.SetNamedProvider(domain, provider); err != nil {
		return nil, err
	}

	return &Flags{client: openfeature.NewClient(domain), enabled: true}, nil
}

func (f *Flags) Enabled(ctx context.Context, key string, fallback bool) bool {
	if f == nil || !f.enabled || f.client == nil {
		return fallback
	}
	return f.client.Boolean(ctx, key, fallback, openfeature.NewTargetlessEvaluationContext(nil))
}

// ClientSafe returns only flags intended for UI behavior. Authorization and
// server-side business rules must never rely on this response.
func (f *Flags) ClientSafe(ctx context.Context) map[string]bool {
	return map[string]bool{
		GraphQLCheckout: f.Enabled(ctx, GraphQLCheckout, true),
		AdminDashboard:  f.Enabled(ctx, AdminDashboard, true),
		MobileCart:      f.Enabled(ctx, MobileCart, true),
	}
}
