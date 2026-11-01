package github_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/changes"
	"github.com/specsnl/gh-release-changes/internal/releasechanges"
)

// fakeGraphQL answers the details query the way GitHub does: one aliased field
// per number, null with a pathed error for a number it cannot resolve.
type fakeGraphQL struct {
	t *testing.T

	issues       map[int]string // number → issue node JSON
	pullRequests map[int]string // number → pull request node JSON

	// missingIssueRepo answers the issues repository itself with NOT_FOUND.
	missingIssueRepo bool

	// status, when set, is the HTTP answer to every request.
	status int

	// unpathedError adds an error that names no field.
	unpathedError bool

	paths     []string
	queries   []string
	variables []map[string]any
}

var aliasRe = regexp.MustCompile(`([ip])(\d+): (?:issue|pullRequest)\(number: (\d+)\)`)

func (f *fakeGraphQL) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.paths = append(f.paths, r.Method+" "+r.URL.Path)

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
	f.checkQuery(request.Query, request.Variables)

	issues, prs := map[string]any{}, map[string]any{}

	var errs []map[string]any

	for _, m := range aliasRe.FindAllStringSubmatch(request.Query, -1) {
		alias := m[1] + m[2]
		n, _ := strconv.Atoi(m[3])

		field, nodes, known, kind := "issues", issues, f.issues, "Issue"
		if m[1] == "p" {
			field, nodes, known, kind = "pullRequests", prs, f.pullRequests, "PullRequest"
		}

		if node, ok := known[n]; ok {
			nodes[alias] = json.RawMessage(node)

			continue
		}

		nodes[alias] = nil

		if m[1] == "i" && f.missingIssueRepo {
			continue
		}

		errs = append(errs, map[string]any{
			"type": "NOT_FOUND", "path": []any{field, alias},
			"message": fmt.Sprintf("Could not resolve to a%s %s with the number of %d.", map[bool]string{true: "n"}[kind == "Issue"], kind, n),
		})
	}

	data := map[string]any{}

	if strings.Contains(request.Query, "issues: repository") {
		data["issues"] = issues

		if f.missingIssueRepo {
			data["issues"] = nil

			errs = append(errs, map[string]any{
				"type": "NOT_FOUND", "path": []any{"issues"},
				"message": "Could not resolve to a Repository with the name 'example-org/gone'.",
			})
		}
	}

	if strings.Contains(request.Query, "pullRequests: repository") {
		data["pullRequests"] = prs
	}

	if f.unpathedError {
		errs = append(errs, map[string]any{"message": "Something went wrong"})
	}

	response := map[string]any{"data": data}
	if len(errs) > 0 {
		response["errors"] = errs
	}

	body, _ := json.Marshal(response)
	reply(w, http.StatusOK, nil, string(body))
}

// checkQuery fails the test on what GitHub would reject outright: a declared
// variable or fragment the query does not use.
func (f *fakeGraphQL) checkQuery(query string, variables map[string]any) {
	for name := range variables {
		if !strings.Contains(query, "$"+name+")") && !strings.Contains(query, "$"+name+",") {
			f.t.Errorf("variable $%s is declared but not used:\n%s", name, query)
		}
	}

	for _, fragment := range []string{"issue", "pullRequest"} {
		if strings.Contains(query, "fragment "+fragment+" ") && !strings.Contains(query, "..."+fragment+" ") {
			f.t.Errorf("fragment %s is defined but not used:\n%s", fragment, query)
		}
	}
}

const issue12 = `{
  "number": 12, "title": "Exports are slow", "body": "Takes minutes.", "state": "CLOSED",
  "url": "https://github.com/example-org/app/issues/12",
  "labels": {"nodes": [{"name": "bug"}, {"name": "export"}]},
  "comments": {"totalCount": 101, "nodes": [{"author": {"login": "bob"}, "body": "Seen it."}, {"author": null, "body": "Me too."}]},
  "closedByPullRequestsReferences": {"nodes": [
    {"number": 40, "title": "feat(12)!: stream", "body": "Streams.", "state": "MERGED",
     "url": "https://github.com/example-org/app/pull/40", "author": {"login": "alice"},
     "repository": {"nameWithOwner": "example-org/app"}},
    null
  ]}
}`

const pr44 = `{
  "number": 44, "title": "feat: dark theme", "body": "Follows the system.", "state": "MERGED",
  "url": "https://github.com/example-org/app/pull/44", "author": {"login": "carol"},
  "repository": {"nameWithOwner": "example-org/app"}
}`

func TestDetails(t *testing.T) {
	fake := &fakeGraphQL{t: t, issues: map[int]string{12: issue12}, pullRequests: map[int]string{44: pr44}}

	details, err := newClientFor(t, fake, "example-org/app").Details(context.Background(), "example-org/app", []int{12, 15}, []int{44, 45})
	if err != nil {
		t.Fatalf("Details: %v", err)
	}

	if len(fake.queries) != 1 {
		t.Errorf("%d queries, want everything in one", len(fake.queries))
	}

	want := changes.Issue{
		Number: 12, Title: "Exports are slow", Body: "Takes minutes.", State: "closed",
		URL:    "https://github.com/example-org/app/issues/12",
		Labels: []string{"bug", "export"},
		Comments: []changes.Comment{
			{Author: "bob", Body: "Seen it."},
			{Author: "ghost", Body: "Me too."},
		},
		CommentCount: 101,
		ClosedBy: []changes.PullRequest{{
			Number: 40, Repo: "example-org/app", Title: "feat(12)!: stream", Body: "Streams.",
			Author: "alice", State: "merged", URL: "https://github.com/example-org/app/pull/40",
		}},
	}

	if got := details.Issues[12]; fmt.Sprintf("%+v", got) != fmt.Sprintf("%+v", want) {
		t.Errorf("issue 12 =\n%+v\nwant\n%+v", got, want)
	}

	if got := details.PullRequests[44]; got.Body != "Follows the system." || got.Author != "carol" || got.State != "merged" {
		t.Errorf("pull request 44 = %+v", got)
	}

	if got := details.UnreadableIssues[15]; got != "Could not resolve to an Issue with the number of 15." {
		t.Errorf("issue 15 reason = %q", got)
	}

	if got := details.UnreadablePullRequests[45]; got != "Could not resolve to a PullRequest with the number of 45." {
		t.Errorf("pull request 45 reason = %q", got)
	}

	vars := fake.variables[0]
	for key, want := range map[string]string{"issueOwner": "example-org", "issueName": "app", "prOwner": "example-org", "prName": "app"} {
		if vars[key] != want {
			t.Errorf("$%s = %v, want %q", key, vars[key], want)
		}
	}
}

// Issues come from --issue-repo, pull requests from the source repository.
func TestDetails_OtherIssueRepo(t *testing.T) {
	fake := &fakeGraphQL{t: t, issues: map[int]string{12: issue12}}

	if _, err := newClientFor(t, fake, "example-org/app").Details(context.Background(), "example-org/tracker", []int{12}, []int{44}); err != nil {
		t.Fatalf("Details: %v", err)
	}

	vars := fake.variables[0]
	if vars["issueName"] != "tracker" || vars["prName"] != "app" {
		t.Errorf("variables = %v, want issues from tracker and pull requests from app", vars)
	}
}

// Only what a batch needs is in its query: GitHub rejects unused variables and
// fragments.
func TestDetails_OnlyIssuesOrOnlyPullRequests(t *testing.T) {
	for _, tc := range []struct {
		name          string
		issues, prs   []int
		wantField     string
		unwantedField string
	}{
		{"only issues", []int{12}, nil, "issues: repository", "pullRequests: repository"},
		{"only pull requests", nil, []int{44}, "pullRequests: repository", "issues: repository"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeGraphQL{t: t, issues: map[int]string{12: issue12}, pullRequests: map[int]string{44: pr44}}

			if _, err := newClientFor(t, fake, "example-org/app").Details(context.Background(), "example-org/app", tc.issues, tc.prs); err != nil {
				t.Fatalf("Details: %v", err)
			}

			query := fake.queries[0]
			if !strings.Contains(query, tc.wantField) || strings.Contains(query, tc.unwantedField) {
				t.Errorf("query:\n%s", query)
			}
		})
	}
}

func TestDetails_NothingToFetch(t *testing.T) {
	fake := &fakeGraphQL{t: t}

	details, err := newClientFor(t, fake, "example-org/app").Details(context.Background(), "example-org/app", nil, nil)
	if err != nil {
		t.Fatalf("Details: %v", err)
	}

	if len(fake.queries) != 0 {
		t.Errorf("%d queries for nothing", len(fake.queries))
	}

	if details.Issues == nil || details.UnreadableIssues == nil {
		t.Error("the maps are nil")
	}
}

func TestDetails_Batches(t *testing.T) {
	fake := &fakeGraphQL{t: t, issues: map[int]string{}, pullRequests: map[int]string{}}

	var issues, prs []int

	for n := 1; n <= 30; n++ {
		issues = append(issues, n)
		fake.issues[n] = fmt.Sprintf(`{"number": %d, "title": "t", "state": "OPEN"}`, n)
	}

	for n := 101; n <= 105; n++ {
		prs = append(prs, n)
		fake.pullRequests[n] = fmt.Sprintf(`{"number": %d, "title": "t", "state": "OPEN"}`, n)
	}

	details, err := newClientFor(t, fake, "example-org/app").Details(context.Background(), "example-org/app", issues, prs)
	if err != nil {
		t.Fatalf("Details: %v", err)
	}

	if len(fake.queries) != 2 {
		t.Errorf("%d queries, want 2 batches", len(fake.queries))
	}

	if len(details.Issues) != 30 || len(details.PullRequests) != 5 {
		t.Errorf("got %d issues and %d pull requests, want 30 and 5", len(details.Issues), len(details.PullRequests))
	}
}

// An issue repository that does not exist makes every issue in it unreadable,
// with the repository's reason; the pull requests are still read.
func TestDetails_MissingIssueRepo(t *testing.T) {
	fake := &fakeGraphQL{t: t, missingIssueRepo: true, pullRequests: map[int]string{44: pr44}}

	details, err := newClientFor(t, fake, "example-org/app").Details(context.Background(), "example-org/gone", []int{12, 13}, []int{44})
	if err != nil {
		t.Fatalf("Details: %v", err)
	}

	for _, n := range []int{12, 13} {
		if got := details.UnreadableIssues[n]; !strings.Contains(got, "Repository") {
			t.Errorf("issue %d reason = %q, want the repository's", n, got)
		}
	}

	if _, ok := details.PullRequests[44]; !ok {
		t.Error("pull request 44 was not read")
	}
}

func TestDetails_Failures(t *testing.T) {
	for _, tc := range []struct {
		name string
		fake fakeGraphQL
		want error
	}{
		{"forbidden", fakeGraphQL{status: http.StatusForbidden}, releasechanges.ErrForbidden},
		{"server error", fakeGraphQL{status: http.StatusBadGateway}, nil},
		{"an error without a path", fakeGraphQL{unpathedError: true}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := tc.fake
			fake.t = t

			_, err := newClientFor(t, &fake, "example-org/app").Details(context.Background(), "example-org/app", []int{12}, nil)
			if err == nil {
				t.Fatal("Details succeeded, want an error")
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

func TestDetails_RejectsAMalformedIssueRepo(t *testing.T) {
	fake := &fakeGraphQL{t: t}

	_, err := newClientFor(t, fake, "example-org/app").Details(context.Background(), "github.com/o/app", []int{1}, nil)
	if !errors.Is(err, releasechanges.ErrInvalidRepoRef) {
		t.Errorf("error = %v, want ErrInvalidRepoRef", err)
	}
}

// GraphQL lives at /graphql on github.com and at /api/graphql on an Enterprise
// host.
func TestDetails_Endpoint(t *testing.T) {
	for _, tc := range []struct{ repo, want string }{
		{"example-org/app", "POST /graphql"},
		{"github.example.com/example-org/app", "POST /api/graphql"},
	} {
		t.Run(tc.repo, func(t *testing.T) {
			fake := &fakeGraphQL{t: t, issues: map[int]string{12: issue12}}

			if _, err := newClientFor(t, fake, tc.repo).Details(context.Background(), "example-org/app", []int{12}, nil); err != nil {
				t.Fatalf("Details: %v", err)
			}

			if !slices.Contains(fake.paths, tc.want) {
				t.Errorf("requests = %v, want %q", fake.paths, tc.want)
			}
		})
	}
}
