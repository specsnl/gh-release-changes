package github_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/specsnl/gh-release-changes/internal/github"
	"github.com/specsnl/gh-release-changes/internal/releasechanges"
)

// fakeGitHub answers the three endpoints the client uses for one repository,
// and records what it was asked.
type fakeGitHub struct {
	t *testing.T

	repo        string
	tags        []string
	missingRepo bool
	emptyRepo   bool

	// tagStatus, when set, is the answer to every tag lookup.
	tagStatus int

	// notesStatus, when set, is the answer to generate-notes, with
	// notesHeaders on the response.
	notesStatus  int
	notesHeaders map[string]string
	body         string

	requests    []string
	notesRequest map[string]any
}

func (f *fakeGitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)

	path := strings.TrimPrefix(r.URL.Path, "/api/v3")
	prefix := "/repos/" + f.repo

	switch {
	case f.missingRepo:
		reply(w, http.StatusNotFound, nil, `{"message":"Not Found"}`)

	case r.Method == http.MethodGet && path == prefix:
		reply(w, http.StatusOK, nil, `{}`)

	case r.Method == http.MethodGet && strings.HasPrefix(path, prefix+"/git/ref/tags/"):
		tag := strings.TrimPrefix(path, prefix+"/git/ref/tags/")

		switch {
		case f.tagStatus != 0:
			reply(w, f.tagStatus, nil, `{"message":"tag lookup refused"}`)
		case f.emptyRepo:
			reply(w, http.StatusConflict, nil, `{"message":"Git Repository is empty."}`)
		case slices.Contains(f.tags, tag):
			reply(w, http.StatusOK, nil, `{"ref":"refs/tags/`+tag+`"}`)
		default:
			reply(w, http.StatusNotFound, nil, `{"message":"Not Found"}`)
		}

	case r.Method == http.MethodPost && path == prefix+"/releases/generate-notes":
		if err := json.NewDecoder(r.Body).Decode(&f.notesRequest); err != nil {
			f.t.Errorf("decode the generate-notes request: %v", err)
		}

		if f.notesStatus != 0 {
			reply(w, f.notesStatus, f.notesHeaders, `{"message":"generate-notes refused"}`)

			return
		}

		response, _ := json.Marshal(map[string]string{"name": "release", "body": f.body})
		reply(w, http.StatusOK, nil, string(response))

	default:
		f.t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		reply(w, http.StatusInternalServerError, nil, `{}`)
	}
}

// calledGenerateNotes reports whether generate-notes was requested at all.
func (f *fakeGitHub) calledGenerateNotes() bool {
	return slices.ContainsFunc(f.requests, func(r string) bool {
		return strings.HasSuffix(r, "/releases/generate-notes")
	})
}

func reply(w http.ResponseWriter, status int, headers map[string]string, body string) {
	w.Header().Set("Content-Type", "application/json")

	for k, v := range headers {
		w.Header().Set(k, v)
	}

	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

// redirect sends every request to the test server, whatever host go-gh built
// the URL for, so the path prefix go-gh picks per host stays observable.
type redirect struct{ to *url.URL }

func (r redirect) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = r.to.Scheme
	req.URL.Host = r.to.Host

	return http.DefaultTransport.RoundTrip(req)
}

// newClient points a Client for repoRef at fake. gh's config and host
// environment are isolated so the developer's own setup cannot leak in.
func newClient(t *testing.T, fake *fakeGitHub, repoRef string) *github.Client {
	t.Helper()

	fake.t = t

	return newClientFor(t, fake, repoRef)
}

// newClientFor is newClient over any handler.
func newClientFor(t *testing.T, handler http.Handler, repoRef string) *github.Client {
	t.Helper()

	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("GH_HOST", "")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	client, err := github.NewWithOptions(repoRef, api.ClientOptions{
		AuthToken:    "test-token",
		Transport:    redirect{to: target},
		LogIgnoreEnv: true,
	})
	if err != nil {
		t.Fatalf("NewWithOptions(%q): %v", repoRef, err)
	}

	return client
}

const finalBody = `## What's Changed
* feat(api): add things by @someone in https://github.com/example-org/app/pull/12

**Full Changelog**: https://github.com/example-org/app/compare/v1.1.0...v1.2.0`

func TestGenerateNotes_GitHubChoosesTheBase(t *testing.T) {
	fake := &fakeGitHub{repo: "example-org/app", tags: []string{"v1.2.0"}, body: finalBody}

	notes, err := newClient(t, fake, "example-org/app").GenerateNotes(context.Background(), "v1.2.0", "")
	if err != nil {
		t.Fatalf("GenerateNotes: %v", err)
	}

	if notes.Body != finalBody {
		t.Errorf("Body = %q, want the generated body as-is", notes.Body)
	}

	if notes.PreviousTag != "v1.1.0" {
		t.Errorf("PreviousTag = %q, want %q from the Full Changelog link", notes.PreviousTag, "v1.1.0")
	}

	if fake.notesRequest["tag_name"] != "v1.2.0" {
		t.Errorf("tag_name = %v, want %q", fake.notesRequest["tag_name"], "v1.2.0")
	}

	if _, ok := fake.notesRequest["previous_tag_name"]; ok {
		t.Errorf("previous_tag_name sent without --previous-tag: %v", fake.notesRequest)
	}
}

func TestGenerateNotes_PreviousTagOverridesTheBase(t *testing.T) {
	fake := &fakeGitHub{repo: "example-org/app", tags: []string{"v1.0.0", "v1.2.0"}, body: finalBody}

	notes, err := newClient(t, fake, "example-org/app").GenerateNotes(context.Background(), "v1.2.0", "v1.0.0")
	if err != nil {
		t.Fatalf("GenerateNotes: %v", err)
	}

	if fake.notesRequest["previous_tag_name"] != "v1.0.0" {
		t.Errorf("previous_tag_name = %v, want %q", fake.notesRequest["previous_tag_name"], "v1.0.0")
	}

	if notes.PreviousTag != "v1.0.0" {
		t.Errorf("PreviousTag = %q, want the one passed in", notes.PreviousTag)
	}

	for _, tag := range []string{"v1.2.0", "v1.0.0"} {
		if !slices.Contains(fake.requests, "GET /repos/example-org/app/git/ref/tags/"+tag) {
			t.Errorf("%s was not checked: %v", tag, fake.requests)
		}
	}
}

func TestGenerateNotes_ResolvesThePreviousTag(t *testing.T) {
	for _, tc := range []struct {
		name, tag, body, want string
	}{
		{"compare link", "v1.2.0", finalBody, "v1.1.0"},
		{"first release", "v1.0.0", "**Full Changelog**: https://github.com/example-org/app/commits/v1.0.0", ""},
		{"no link at all", "v1.0.0", "## What's Changed\n", ""},
		{"escaped tag", "release/2.0", "**Full Changelog**: https://github.com/example-org/app/compare/release%2F1.0...release%2F2.0", "release/1.0"},
		{"pre-release", "v2.0.0-rc.2", "**Full Changelog**: https://github.com/example-org/app/compare/v2.0.0-rc.1...v2.0.0-rc.2\n", "v2.0.0-rc.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeGitHub{repo: "example-org/app", tags: []string{tc.tag}, body: tc.body}

			notes, err := newClient(t, fake, "example-org/app").GenerateNotes(context.Background(), tc.tag, "")
			if err != nil {
				t.Fatalf("GenerateNotes: %v", err)
			}

			if notes.PreviousTag != tc.want {
				t.Errorf("PreviousTag = %q, want %q", notes.PreviousTag, tc.want)
			}
		})
	}
}

// A tag with a slash is a nested ref: the separator stays a separator.
func TestGenerateNotes_TagWithASlash(t *testing.T) {
	fake := &fakeGitHub{repo: "example-org/app", tags: []string{"release/2.0"}, body: finalBody}

	if _, err := newClient(t, fake, "example-org/app").GenerateNotes(context.Background(), "release/2.0", ""); err != nil {
		t.Fatalf("GenerateNotes: %v", err)
	}
}

// The host of -R decides where requests go: a GitHub Enterprise host is
// served under /api/v3.
func TestGenerateNotes_EnterpriseHost(t *testing.T) {
	fake := &fakeGitHub{repo: "example-org/app", tags: []string{"v1.2.0"}, body: finalBody}

	client := newClient(t, fake, "github.example.com/example-org/app")
	if _, err := client.GenerateNotes(context.Background(), "v1.2.0", ""); err != nil {
		t.Fatalf("GenerateNotes: %v", err)
	}

	if want := "POST /api/v3/repos/example-org/app/releases/generate-notes"; !slices.Contains(fake.requests, want) {
		t.Errorf("requests = %v, want %q", fake.requests, want)
	}

	if client.Repo() != "example-org/app" {
		t.Errorf("Repo() = %q, want %q", client.Repo(), "example-org/app")
	}
}

func TestGenerateNotes_ErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		name        string
		fake        fakeGitHub
		tag         string
		previousTag string

		// want is the sentinel, or nil for an error that must not be mapped.
		want     error
		wantKind string

		// notesCalled is whether generate-notes may be reached at all.
		notesCalled bool
	}{
		{
			name: "unknown tag",
			fake: fakeGitHub{tags: []string{"v1.2.0"}},
			tag:  "v1.2.1", want: releasechanges.ErrTagNotFound, wantKind: "tag_not_found",
		},
		{
			name: "unknown previous tag",
			fake: fakeGitHub{tags: []string{"v1.2.0"}},
			tag:  "v1.2.0", previousTag: "v0.9.9", want: releasechanges.ErrTagNotFound, wantKind: "tag_not_found",
		},
		{
			name: "empty repository",
			fake: fakeGitHub{emptyRepo: true},
			tag:  "v1.2.0", want: releasechanges.ErrTagNotFound, wantKind: "tag_not_found",
		},
		{
			name: "missing repository",
			fake: fakeGitHub{missingRepo: true},
			tag:  "v1.2.0", want: releasechanges.ErrRepoNotFound, wantKind: "repo_not_found",
		},
		{
			name: "tag lookup forbidden",
			fake: fakeGitHub{tagStatus: http.StatusForbidden},
			tag:  "v1.2.0", want: releasechanges.ErrForbidden, wantKind: "forbidden",
		},
		{
			name: "generate-notes forbidden",
			fake: fakeGitHub{tags: []string{"v1.2.0"}, notesStatus: http.StatusForbidden},
			tag:  "v1.2.0", want: releasechanges.ErrForbidden, wantKind: "forbidden", notesCalled: true,
		},
		{
			name: "generate-notes not found",
			fake: fakeGitHub{tags: []string{"v1.2.0"}, notesStatus: http.StatusNotFound},
			tag:  "v1.2.0", want: releasechanges.ErrRepoNotFound, wantKind: "repo_not_found", notesCalled: true,
		},
		{
			name: "rate limited",
			fake: fakeGitHub{
				tags: []string{"v1.2.0"}, notesStatus: http.StatusForbidden,
				notesHeaders: map[string]string{"X-RateLimit-Remaining": "0"},
			},
			tag: "v1.2.0", notesCalled: true,
		},
		{
			name: "server error",
			fake: fakeGitHub{tags: []string{"v1.2.0"}, notesStatus: http.StatusBadGateway},
			tag:  "v1.2.0", notesCalled: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := tc.fake
			fake.repo = "example-org/app"

			_, err := newClient(t, &fake, "example-org/app").GenerateNotes(context.Background(), tc.tag, tc.previousTag)
			if err == nil {
				t.Fatal("GenerateNotes succeeded, want an error")
			}

			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}

			if kind := releasechanges.KindOf(err); kind != tc.wantKind {
				t.Errorf("kind = %q, want %q (error: %v)", kind, tc.wantKind, err)
			}

			if tc.want == nil {
				var httpErr *api.HTTPError
				if !errors.As(err, &httpErr) {
					t.Errorf("an unmapped error lost go-gh's HTTPError: %v", err)
				}
			}

			if called := fake.calledGenerateNotes(); called != tc.notesCalled {
				t.Errorf("generate-notes called = %v, want %v (requests: %v)", called, tc.notesCalled, fake.requests)
			}
		})
	}
}

func TestNew_RejectsAMalformedRepo(t *testing.T) {
	t.Setenv("GH_CONFIG_DIR", t.TempDir())

	_, err := github.NewWithOptions("a/b/c/d", api.ClientOptions{AuthToken: "test-token"})
	if !errors.Is(err, releasechanges.ErrInvalidRepoRef) {
		t.Errorf("error = %v, want ErrInvalidRepoRef", err)
	}
}
