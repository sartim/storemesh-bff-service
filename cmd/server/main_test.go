package main

import (
	"encoding/base64"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
)

func TestProductGraphQLSchema(t *testing.T) {
	result := graphql.Do(graphql.Params{Schema: productGraphQLSchema, RequestString: `{ __typename }`})
	if len(result.Errors) != 0 {
		t.Fatalf("schema execution returned errors: %v", result.Errors)
	}
	if result.Data.(map[string]interface{})["__typename"] != "Query" {
		t.Fatalf("unexpected root type: %#v", result.Data)
	}
}

func TestGraphQLCommerceContract(t *testing.T) {
	result := graphql.Do(graphql.Params{Schema: productGraphQLSchema, RequestString: `{ __schema { queryType { fields { name } } mutationType { fields { name args { name type { kind name ofType { kind name } } } } } } }`})
	if len(result.Errors) != 0 {
		t.Fatalf("schema introspection returned errors: %v", result.Errors)
	}
	data := result.Data.(map[string]interface{})["__schema"].(map[string]interface{})
	queryNames := fieldNames(data["queryType"].(map[string]interface{})["fields"])
	for _, name := range []string{"products", "cart", "orders"} {
		if !queryNames[name] {
			t.Fatalf("query field %q is missing", name)
		}
	}
	mutationNames := fieldNames(data["mutationType"].(map[string]interface{})["fields"])
	for _, name := range []string{"updateCart", "clearCart", "createOrder"} {
		if !mutationNames[name] {
			t.Fatalf("mutation field %q is missing", name)
		}
	}
}

func fieldNames(value interface{}) map[string]bool {
	names := map[string]bool{}
	for _, item := range value.([]interface{}) {
		name := item.(map[string]interface{})["name"].(string)
		names[name] = true
	}
	return names
}

func TestGraphQLRejectsInvalidRequest(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/v1/graphql", strings.NewReader("not-json"))
	(&server{}).graphQL(recorder, request)
	if recorder.Code != 400 {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}

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
