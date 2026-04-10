package example

import (
	"context"

	elasticsearch "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/sortorder"
)

type esClient struct {
	client *elasticsearch.TypedClient
}

func newESClient(url string) (*esClient, error) {
	cfg := elasticsearch.Config{
		Addresses: []string{url},
	}
	c, err := elasticsearch.NewTypedClient(cfg)
	if err != nil {
		return nil, err
	}
	return &esClient{client: c}, nil
}

// Search executes a search request against the given index.
func (c *esClient) Search(ctx context.Context, index string, req *search.Request) (*search.Response, error) {
	return c.client.Search().
		Index(index).
		Request(req).
		Do(ctx)
}

// BuildActiveUsersQuery builds a request for active users matching the given name.
// bool.must[match(name)] + bool.filter[term(status:"active")]
func BuildActiveUsersQuery(name string) *search.Request {
	return &search.Request{
		Query: &types.Query{
			Bool: &types.BoolQuery{
				Must: []types.Query{
					{
						Match: map[string]types.MatchQuery{
							"name": {Query: name},
						},
					},
				},
				Filter: []types.Query{
					{
						Term: map[string]types.TermQuery{
							"status": {Value: "active"},
						},
					},
				},
			},
		},
		Sort: []types.SortCombinations{sortByField("email")},
	}
}

// BuildPriceRangeQuery builds a request for products in a price range.
// bool.must[match(category)] + bool.filter[range(price: gte/lte)]
func BuildPriceRangeQuery(category string, minPrice, maxPrice float64) *search.Request {
	min := types.Float64(minPrice)
	max := types.Float64(maxPrice)
	return &search.Request{
		Query: &types.Query{
			Bool: &types.BoolQuery{
				Must: []types.Query{
					{
						Match: map[string]types.MatchQuery{
							"category": {Query: category},
						},
					},
				},
				Filter: []types.Query{
					{
						Range: map[string]types.RangeQuery{
							"price": types.NumberRangeQuery{
								Gte: &min,
								Lte: &max,
							},
						},
					},
				},
			},
		},
		Sort: []types.SortCombinations{sortByField("price")},
	}
}

// BuildArticlesQuery builds a request for non-draft articles matching the given title.
// bool.must[match(title)] + bool.must_not[term(status:"draft")]
func BuildArticlesQuery(title string) *search.Request {
	return &search.Request{
		Query: &types.Query{
			Bool: &types.BoolQuery{
				Must: []types.Query{
					{
						Match: map[string]types.MatchQuery{
							"title": {Query: title},
						},
					},
				},
				MustNot: []types.Query{
					{
						Term: map[string]types.TermQuery{
							"status": {Value: "draft"},
						},
					},
				},
			},
		},
		Sort: []types.SortCombinations{sortByField("title.keyword")},
	}
}

// BuildUserByEmailQuery builds a request for a user with the given email.
// bool.must[term(email:...)]
func BuildUserByEmailQuery(email string) *search.Request {
	return &search.Request{
		Query: &types.Query{
			Bool: &types.BoolQuery{
				Must: []types.Query{
					{
						Term: map[string]types.TermQuery{
							"email": {Value: email},
						},
					},
				},
			},
		},
	}
}

func sortByField(field string) types.SortCombinations {
	order := sortorder.Asc
	return types.SortOptions{
		SortOptions: map[string]types.FieldSort{
			field: {Order: &order},
		},
	}
}
