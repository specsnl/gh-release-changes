package github_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/specsnl/gh-release-changes/internal/github"
	"github.com/specsnl/gh-release-changes/internal/releasechanges"
)

// fakeTickets answers the contents and issues endpoints of example-org/app,
// and records what it was asked.
type fakeTickets struct {
	t *testing.T

	// files are the files of .github/release-changes on the default branch;
	// nil answers the listing with a 404.
	files map[string]string

	// issues are the repository's issues, served in pages of 100.
	issues []map[string]any

	// status, when set, is the answer to every issues request.
	status int

	requests []string
	queries  []string
	payload  map[string]any
}

const templatesPath = "/repos/example-org/app/contents/.github/release-changes"

func (f *fakeTickets) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.requests = append(f.requests, r.Method+" "+r.URL.Path)

	switch {
	case r.Method == http.MethodGet && r.URL.Path == templatesPath:
		if f.files == nil {
			reply(w, http.StatusNotFound, nil, `{"message":"Not Found"}`)

			return
		}

		var listing []map[string]string

		for name := range f.files {
			listing = append(listing, map[string]string{"name": name, "path": ".github/release-changes/" + name, "type": "file"})
		}

		listing = append(listing, map[string]string{"name": "nested.tmpl", "path": ".github/release-changes/nested.tmpl", "type": "dir"})

		f.json(w, http.StatusOK, listing)

	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, templatesPath+"/"):
		name := strings.TrimPrefix(r.URL.Path, templatesPath+"/")

		// GitHub wraps the base64 content at 60 characters.
		encoded := base64.StdEncoding.EncodeToString([]byte(f.files[name]))
		for i := 60; i < len(encoded); i += 61 {
			encoded = encoded[:i] + "\n" + encoded[i:]
		}

		f.json(w, http.StatusOK, map[string]string{"content": encoded, "encoding": "base64"})

	case f.status != 0:
		reply(w, f.status, nil, `{"message":"refused"}`)

	case r.Method == http.MethodGet && r.URL.Path == "/repos/example-org/app/issues":
		f.queries = append(f.queries, r.URL.RawQuery)

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		from := min((page-1)*100, len(f.issues))
		f.json(w, http.StatusOK, f.issues[from:min(from+100, len(f.issues))])

	case r.Method == http.MethodPost && r.URL.Path == "/repos/example-org/app/issues",
		r.Method == http.MethodPatch && r.URL.Path == "/repos/example-org/app/issues/12":
		if err := json.NewDecoder(r.Body).Decode(&f.payload); err != nil {
			f.t.Errorf("decode the request: %v", err)
		}

		f.json(w, http.StatusOK, map[string]any{
			"number": 12, "title": f.payload["title"], "body": f.payload["body"],
			"html_url": "https://github.com/example-org/app/issues/12",
		})

	default:
		f.t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		reply(w, http.StatusInternalServerError, nil, `{}`)
	}
}

func (f *fakeTickets) json(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		f.t.Fatal(err)
	}

	reply(w, status, nil, string(body))
}

func ticketClient(t *testing.T, fake *fakeTickets) *github.Client {
	t.Helper()

	fake.t = t

	return newClientFor(t, fake, "example-org/app")
}

func TestTicketTemplates_ReadsTheTmplFiles(t *testing.T) {
	fake := &fakeTickets{files: map[string]string{
		"ticket.md.tmpl":    "---\ntitle: Release {{ .Version }}\n---\n" + strings.Repeat("a long line of the template\n", 10),
		"checklist.md.tmpl": "- [ ] tested\n",
		"README.md":         "not a part",
	}}

	files, err := ticketClient(t, fake).TicketTemplates(context.Background(), ".github/release-changes")
	if err != nil {
		t.Fatalf("TicketTemplates: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("files = %v, want the two .tmpl files", files)
	}

	for _, name := range []string{"ticket.md.tmpl", "checklist.md.tmpl"} {
		if files[name] != fake.files[name] {
			t.Errorf("%s = %q, want %q", name, files[name], fake.files[name])
		}
	}

	if want := 3; len(fake.requests) != want {
		t.Errorf("requests = %v, want one listing and one per .tmpl file", fake.requests)
	}
}

func TestTicketTemplates_NoDirectory(t *testing.T) {
	files, err := ticketClient(t, &fakeTickets{}).TicketTemplates(context.Background(), ".github/release-changes")
	if err != nil {
		t.Fatalf("TicketTemplates: %v", err)
	}

	if files != nil {
		t.Errorf("files = %v, want nil", files)
	}
}

func issue(number int, title string) map[string]any {
	return map[string]any{
		"number": number, "title": title, "body": "body of " + title,
		"html_url": fmt.Sprintf("https://github.com/example-org/app/issues/%d", number),
	}
}

func TestTickets_FollowsThePagesAndSkipsPullRequests(t *testing.T) {
	fake := &fakeTickets{}

	for n := 1; n <= 150; n++ {
		fake.issues = append(fake.issues, issue(n, fmt.Sprintf("Release 1.%d.0", n)))
	}

	fake.issues[3]["pull_request"] = map[string]any{"url": "x"}

	tickets, err := ticketClient(t, fake).Tickets(context.Background(), []string{"release", "woo"})
	if err != nil {
		t.Fatalf("Tickets: %v", err)
	}

	if len(tickets) != 149 {
		t.Errorf("tickets = %d, want 149: 150 issues, one of them a pull request", len(tickets))
	}

	want := github.Ticket{Number: 1, Title: "Release 1.1.0", Body: "body of Release 1.1.0", URL: "https://github.com/example-org/app/issues/1"}
	if tickets[0] != want {
		t.Errorf("tickets[0] = %+v, want %+v", tickets[0], want)
	}

	if len(fake.queries) != 2 {
		t.Fatalf("queries = %v, want two pages", fake.queries)
	}

	if want := "labels=release%2Cwoo&page=1&per_page=100&state=all"; fake.queries[0] != want {
		t.Errorf("query = %q, want %q", fake.queries[0], want)
	}
}

func TestCreateTicket(t *testing.T) {
	fake := &fakeTickets{}

	ticket, err := ticketClient(t, fake).CreateTicket(context.Background(), "Release 1.0.0", "the body", []string{"release"})
	if err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}

	if ticket.Number != 12 || ticket.URL != "https://github.com/example-org/app/issues/12" {
		t.Errorf("ticket = %+v, want #12 with its URL", ticket)
	}

	want := map[string]any{"title": "Release 1.0.0", "body": "the body", "labels": []any{"release"}}
	if fmt.Sprint(fake.payload) != fmt.Sprint(want) {
		t.Errorf("payload = %v, want %v", fake.payload, want)
	}
}

// An update leaves the labels alone: the ticket was found by them.
func TestUpdateTicket(t *testing.T) {
	fake := &fakeTickets{}

	if _, err := ticketClient(t, fake).UpdateTicket(context.Background(), 12, "Release 1.0.0", "the body"); err != nil {
		t.Fatalf("UpdateTicket: %v", err)
	}

	if want := "PATCH /repos/example-org/app/issues/12"; fake.requests[0] != want {
		t.Errorf("request = %q, want %q", fake.requests[0], want)
	}

	if _, ok := fake.payload["labels"]; ok {
		t.Errorf("payload = %v, want no labels", fake.payload)
	}
}

// A 403 on the issues says it needs Issues: write, not Contents: write; the
// kind is the same.
func TestTickets_ErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		want    error
		message string
	}{
		{"forbidden", http.StatusForbidden, releasechanges.ErrForbidden, "Issues: write"},
		{"not found", http.StatusNotFound, releasechanges.ErrRepoNotFound, "example-org/app"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := ticketClient(t, &fakeTickets{status: tc.status})

			for name, call := range map[string]func() error{
				"Tickets": func() error {
					_, err := client.Tickets(context.Background(), []string{"release"})

					return err
				},
				"CreateTicket": func() error {
					_, err := client.CreateTicket(context.Background(), "t", "b", nil)

					return err
				},
				"UpdateTicket": func() error {
					_, err := client.UpdateTicket(context.Background(), 12, "t", "b")

					return err
				},
			} {
				err := call()
				if !errors.Is(err, tc.want) {
					t.Errorf("%s: error = %v, want %v", name, err, tc.want)
				}

				if !strings.Contains(fmt.Sprint(err), tc.message) {
					t.Errorf("%s: error = %v, want it to mention %q", name, err, tc.message)
				}
			}
		})
	}
}
