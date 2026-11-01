package github_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/releasechanges"
)

// fakeTags pages through tags the way GitHub's refs connection does, with the
// cursor being the index of the next tag.
type fakeTags struct {
	t *testing.T

	tags        []string
	missingRepo bool
	status      int

	requests []map[string]any
}

func (f *fakeTags) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if f.status != 0 {
		reply(w, f.status, nil, `{"message":"refused"}`)

		return
	}

	var request struct {
		Variables map[string]any `json:"variables"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		f.t.Errorf("decode the GraphQL request: %v", err)
	}

	f.requests = append(f.requests, request.Variables)

	if f.missingRepo {
		reply(w, http.StatusOK, nil, `{"data":{"repository":null},"errors":[{"type":"NOT_FOUND","path":["repository"],"message":"Could not resolve to a Repository with the name 'example-org/gone'."}]}`)

		return
	}

	start := 0
	if after, ok := request.Variables["after"].(string); ok {
		start, _ = strconv.Atoi(after)
	}

	end := min(start+int(request.Variables["first"].(float64)), len(f.tags))

	nodes := []map[string]string{}
	for _, tag := range f.tags[start:end] {
		nodes = append(nodes, map[string]string{"name": tag})
	}

	body, _ := json.Marshal(map[string]any{"data": map[string]any{"repository": map[string]any{"refs": map[string]any{
		"nodes":    nodes,
		"pageInfo": map[string]any{"hasNextPage": end < len(f.tags), "endCursor": strconv.Itoa(end)},
	}}}})
	reply(w, http.StatusOK, nil, string(body))
}

func TestTags_FollowsThePages(t *testing.T) {
	fake := &fakeTags{t: t}
	for n := range 250 {
		fake.tags = append(fake.tags, "v1.0."+strconv.Itoa(n))
	}

	tags, err := newClientFor(t, fake, "example-org/app").Tags(context.Background())
	if err != nil {
		t.Fatalf("Tags: %v", err)
	}

	if !slices.Equal(tags, fake.tags) {
		t.Errorf("got %d tags, want all %d in GitHub's order", len(tags), len(fake.tags))
	}

	if len(fake.requests) != 3 {
		t.Errorf("%d requests, want 3 pages", len(fake.requests))
	}

	if first := fake.requests[0]; first["owner"] != "example-org" || first["name"] != "app" || first["after"] != nil {
		t.Errorf("first request variables = %v", first)
	}
}

func TestTags_None(t *testing.T) {
	tags, err := newClientFor(t, &fakeTags{t: t}, "example-org/app").Tags(context.Background())
	if err != nil {
		t.Fatalf("Tags: %v", err)
	}

	if tags == nil || len(tags) != 0 {
		t.Errorf("Tags = %#v, want an empty list", tags)
	}
}

func TestTags_Failures(t *testing.T) {
	for _, tc := range []struct {
		name string
		fake fakeTags
		want error
	}{
		{"missing repository", fakeTags{missingRepo: true}, releasechanges.ErrRepoNotFound},
		{"forbidden", fakeTags{status: http.StatusForbidden}, releasechanges.ErrForbidden},
		{"server error", fakeTags{status: http.StatusBadGateway}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := tc.fake
			fake.t = t

			_, err := newClientFor(t, &fake, "example-org/app").Tags(context.Background())
			if err == nil {
				t.Fatal("Tags succeeded, want an error")
			}

			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}

			if tc.want == nil && releasechanges.KindOf(err) != "" {
				t.Errorf("error = %v, want it unmapped", err)
			}
		})
	}
}
