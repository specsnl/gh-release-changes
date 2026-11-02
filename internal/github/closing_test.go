package github_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/releasechanges"
)

// fakeClosing answers the closing-references query the way GitHub does: one
// aliased pullRequest per number, null with a pathed error for a number it
// cannot resolve.
type fakeClosing struct {
	t *testing.T

	// closes maps a pull request to the OWNER/REPO#N issues it closes.
	closes map[int][]string

	// total, when set for a pull request, is its totalCount.
	total map[int]int

	status int

	queries   []string
	variables []map[string]any
}

var closingAliasRe = regexp.MustCompile(`(p\d+): pullRequest\(number: (\d+)\) \{ \.\.\.closing \}`)

func (f *fakeClosing) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if f.status != 0 {
		reply(w, f.status, nil, `{"message":"refused"}`)

		return
	}

	var request struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		f.t.Errorf("decode the GraphQL request: %v", err)
	}

	f.queries = append(f.queries, request.Query)
	f.variables = append(f.variables, request.Variables)

	prs := map[string]any{}

	var errs []map[string]any

	for _, m := range closingAliasRe.FindAllStringSubmatch(request.Query, -1) {
		n, _ := strconv.Atoi(m[2])

		closes, ok := f.closes[n]
		if !ok {
			prs[m[1]] = nil
			errs = append(errs, map[string]any{
				"type": "NOT_FOUND", "path": []any{"repository", m[1]},
				"message": fmt.Sprintf("Could not resolve to a PullRequest with the number of %d.", n),
			})

			continue
		}

		nodes := []any{}

		for _, ref := range closes {
			repo, number, _ := strings.Cut(ref, "#")
			num, _ := strconv.Atoi(number)
			nodes = append(nodes, map[string]any{"number": num, "repository": map[string]any{"nameWithOwner": repo}})
		}

		total := len(nodes)
		if t, ok := f.total[n]; ok {
			total = t
		}

		prs[m[1]] = map[string]any{"closingIssuesReferences": map[string]any{"totalCount": total, "nodes": nodes}}
	}

	response := map[string]any{"data": map[string]any{"repository": prs}}
	if len(errs) > 0 {
		response["errors"] = errs
	}

	body, _ := json.Marshal(response)
	reply(w, http.StatusOK, nil, string(body))
}

func TestClosingIssues(t *testing.T) {
	fake := &fakeClosing{t: t, closes: map[int][]string{
		40: {"example-org/app#12", "example-org/app#13"},
		41: {},
		42: {"Example-Org/App#7", "example-org/elsewhere#99"},
	}}

	got, err := newClientFor(t, fake, "example-org/app").ClosingIssues(context.Background(), "example-org/app", []int{40, 41, 42})
	if err != nil {
		t.Fatalf("ClosingIssues: %v", err)
	}

	// #41 closes nothing; another repository's issue is left out; the
	// repository is compared without regard to case.
	if want := map[int][]int{40: {12, 13}, 42: {7}}; !reflect.DeepEqual(got, want) {
		t.Errorf("closing = %v, want %v", got, want)
	}

	if len(fake.queries) != 1 {
		t.Fatalf("made %d queries, want 1", len(fake.queries))
	}

	if want := map[string]any{"owner": "example-org", "name": "app"}; !reflect.DeepEqual(fake.variables[0], want) {
		t.Errorf("variables = %v, want %v", fake.variables[0], want)
	}

	if !strings.Contains(fake.queries[0], "closingIssuesReferences(first: 25)") {
		t.Errorf("query does not read the closing references:\n%s", fake.queries[0])
	}
}

// The issues may live in another repository than the pull requests.
func TestClosingIssues_OtherIssueRepo(t *testing.T) {
	fake := &fakeClosing{t: t, closes: map[int][]string{40: {"example-org/app#12", "example-org/tracker#5"}}}

	got, err := newClientFor(t, fake, "example-org/app").ClosingIssues(context.Background(), "example-org/tracker", []int{40})
	if err != nil {
		t.Fatalf("ClosingIssues: %v", err)
	}

	if want := map[int][]int{40: {5}}; !reflect.DeepEqual(got, want) {
		t.Errorf("closing = %v, want %v", got, want)
	}
}

func TestClosingIssues_Batches(t *testing.T) {
	fake := &fakeClosing{t: t, closes: map[int][]string{}}

	var prs []int
	for n := 1; n <= 150; n++ {
		prs = append(prs, n)
		fake.closes[n] = []string{"example-org/app#" + strconv.Itoa(1000+n)}
	}

	got, err := newClientFor(t, fake, "example-org/app").ClosingIssues(context.Background(), "example-org/app", prs)
	if err != nil {
		t.Fatalf("ClosingIssues: %v", err)
	}

	if len(fake.queries) != 2 {
		t.Errorf("made %d queries, want 2 for 150 pull requests", len(fake.queries))
	}

	if len(got) != 150 || !reflect.DeepEqual(got[150], []int{1150}) {
		t.Errorf("got %d pull requests, #150 → %v; want 150, #150 → [1150]", len(got), got[150])
	}
}

func TestClosingIssues_NothingToFetch(t *testing.T) {
	fake := &fakeClosing{t: t}

	got, err := newClientFor(t, fake, "example-org/app").ClosingIssues(context.Background(), "example-org/app", nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("ClosingIssues = %v, %v; want nothing", got, err)
	}

	if len(fake.queries) != 0 {
		t.Errorf("made %d queries, want none", len(fake.queries))
	}
}

// More references than one page holds are left out, not an error.
func TestClosingIssues_MoreThanAPage(t *testing.T) {
	fake := &fakeClosing{t: t, closes: map[int][]string{40: {"example-org/app#12"}}, total: map[int]int{40: 30}}

	got, err := newClientFor(t, fake, "example-org/app").ClosingIssues(context.Background(), "example-org/app", []int{40})
	if err != nil {
		t.Fatalf("ClosingIssues: %v", err)
	}

	if want := map[int][]int{40: {12}}; !reflect.DeepEqual(got, want) {
		t.Errorf("closing = %v, want %v", got, want)
	}
}

func TestClosingIssues_Failures(t *testing.T) {
	for _, tc := range []struct {
		name    string
		fake    fakeClosing
		want    error
		message string
	}{
		{"forbidden", fakeClosing{status: http.StatusForbidden}, releasechanges.ErrForbidden, ""},
		{"server error", fakeClosing{status: http.StatusBadGateway}, nil, ""},
		{"an unreadable pull request", fakeClosing{closes: map[int][]string{}}, nil,
			"read the closing references of pull request #40 of example-org/app: Could not resolve to a PullRequest with the number of 40."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := tc.fake
			fake.t = t

			_, err := newClientFor(t, &fake, "example-org/app").ClosingIssues(context.Background(), "example-org/app", []int{40})
			if err == nil {
				t.Fatal("ClosingIssues succeeded, want an error")
			}

			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}

			if tc.want == nil && releasechanges.KindOf(err) != "" {
				t.Errorf("error = %v, want it unmapped", err)
			}

			if tc.message != "" && err.Error() != tc.message {
				t.Errorf("error = %q, want %q", err, tc.message)
			}
		})
	}
}

func TestClosingIssues_RejectsAMalformedIssueRepo(t *testing.T) {
	fake := &fakeClosing{t: t}

	_, err := newClientFor(t, fake, "example-org/app").ClosingIssues(context.Background(), "example-org", []int{40})
	if !errors.Is(err, releasechanges.ErrInvalidRepoRef) {
		t.Errorf("error = %v, want ErrInvalidRepoRef", err)
	}
}
