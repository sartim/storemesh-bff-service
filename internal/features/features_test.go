package features

import (
	"context"
	"testing"
)

func TestDisabledProviderUsesSafeDefaults(t *testing.T) {
	flags, err := New("", "")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if !flags.Enabled(context.Background(), GraphQLCheckout, true) {
		t.Fatal("expected fallback value to be used")
	}
	if flags.Enabled(context.Background(), KafkaAnalytics, false) {
		t.Fatal("expected disabled fallback value to be used")
	}
	if len(flags.ClientSafe(context.Background())) != 3 {
		t.Fatal("expected only client-safe flags to be exposed")
	}
}
