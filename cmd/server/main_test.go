package main

import (
	"encoding/base64"
	"testing"
)

func TestPathID(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		prefix string
		want   string
	}{
		{name: "matching collection", path: "/api/v1/products/", prefix: "/api/v1/products/", want: ""},
		{name: "matching resource", path: "/api/v1/products/p-123", prefix: "/api/v1/products/", want: "p-123"},
		{name: "similar prefix does not match", path: "/api/v1/products-extra/p-123", prefix: "/api/v1/products/", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pathID(tt.path, tt.prefix); got != tt.want {
				t.Fatalf("pathID(%q, %q) = %q, want %q", tt.path, tt.prefix, got, tt.want)
			}
		})
	}
}

func TestBearerSubject(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"customer-123"}`))
	token := "header." + payload + ".signature"
	if got := bearerSubject("Bearer " + token); got != "customer-123" {
		t.Fatalf("bearerSubject() = %q", got)
	}
	if got := bearerSubject("Bearer invalid"); got != "" {
		t.Fatalf("invalid token subject = %q", got)
	}
}
