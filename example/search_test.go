package example

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	elasticsearch "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/google/go-cmp/cmp"
	testfixtures "github.com/kurakura967/go-elasticsearch-testfixtures"
)

const (
	indexUsers    = "users"
	indexProducts = "products"
	indexArticles = "articles"
)

var testClient *esClient

func TestMain(m *testing.M) {
	url := os.Getenv("ELASTICSEARCH_URL")
	if url == "" {
		url = "http://localhost:9200"
	}
	cfg := elasticsearch.Config{Addresses: []string{url}}

	rawClient, err := elasticsearch.NewClient(cfg)
	if err != nil {
		panic(err)
	}

	typedClient, err := elasticsearch.NewTypedClient(cfg)
	if err != nil {
		panic(err)
	}
	testClient = &esClient{client: typedClient}

	loader, err := testfixtures.New(rawClient, testfixtures.Directory("testdata/fixtures"))
	if err != nil {
		panic(err)
	}

	if err := loader.Load(); err != nil {
		panic(err)
	}

	code := m.Run()

	loader.Clean() //nolint:errcheck
	os.Exit(code)
}

// extractDocs extracts the source document body from each hit in the response.
func extractDocs(resp *search.Response) []map[string]any {
	docs := make([]map[string]any, 0, len(resp.Hits.Hits))
	for _, hit := range resp.Hits.Hits {
		var doc map[string]any
		if err := json.Unmarshal(hit.Source_, &doc); err != nil {
			continue
		}
		docs = append(docs, doc)
	}
	return docs
}

// TestBuildActiveUsersQuery verifies that only active users matching the name are returned.
// MustToShould mutation: user-4, user-5 would appear in results.
// RemoveClause mutation: all active users regardless of name would appear.
func TestBuildActiveUsersQuery(t *testing.T) {
	resp, err := testClient.Search(context.Background(), indexUsers, BuildActiveUsersQuery("Alice"))
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	got := extractDocs(resp)
	want := []map[string]any{
		{"name": "Alice Brown", "status": "active", "email": "alice2@example.com", "username": "alice_b"},
		{"name": "Alice Smith", "status": "active", "email": "alice@example.com", "username": "alice_s"},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

// TestBuildPriceRangeQuery verifies that boundary values are included.
// RangeBoundary mutation: gte→gt drops product-1 (price=100), lte→lt drops product-3 (price=300).
func TestBuildPriceRangeQuery(t *testing.T) {
	resp, err := testClient.Search(context.Background(), indexProducts, BuildPriceRangeQuery("electronics", 100.0, 300.0))
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	got := extractDocs(resp)
	want := []map[string]any{
		{"category": "electronics", "price": float64(100)},
		{"category": "electronics", "price": float64(200)},
		{"category": "electronics", "price": float64(300)},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

// TestBuildArticlesQuery verifies that draft articles are excluded.
// RemoveMustNot mutation: article-3 (draft) would appear in results.
func TestBuildArticlesQuery(t *testing.T) {
	resp, err := testClient.Search(context.Background(), indexArticles, BuildArticlesQuery("Go"))
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	got := extractDocs(resp)
	want := []map[string]any{
		{"title": "Go Programming", "status": "published"},
		{"title": "Go Programming Guide", "status": "published"},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

// TestBuildUserByEmailQuery verifies that the correct user is found by email.
// SwapField mutation: swapping "email" to "username" returns no results for this value.
func TestBuildUserByEmailQuery(t *testing.T) {
	resp, err := testClient.Search(context.Background(), indexUsers, BuildUserByEmailQuery("alice@example.com"))
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	got := extractDocs(resp)
	want := []map[string]any{
		{"name": "Alice Smith", "status": "active", "email": "alice@example.com", "username": "alice_s"},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}
