package bot

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/google/go-github/github"
)

type ClosingBuffer struct {
	*bytes.Buffer
}

func (cb *ClosingBuffer) Close() error {
	return nil
}

var issueOpenedRequest = &http.Request{
	Body: &ClosingBuffer{bytes.NewBufferString(`
		{
			"action": "opened",
			"issue": {
				"number": 1234,
				"labels": []
			},
			"repository": {
				"default_branch": "master",
				"name": "Fredrik",
				"owner": {
					"login": "test"
				}
			},
			"sender": {
				"login": "test"
			},
			"installation": {
				"id": 1234
			}
		}
	`)},
}

var issueCreatedRequest = &http.Request{
	Body: &ClosingBuffer{bytes.NewBufferString(`
		{
			"action": "created",
			"issue": {
				"number": 1234,
				"labels": []
			},
			"repository": {
				"default_branch": "master",
				"name": "Fredrik",
				"owner": {
					"login": "test"
				}
			},
			"sender": {
				"login": "test"
			},
			"installation": {
				"id": 1234
			}
		}
	`)},
}

type githubIssues struct {
}

func (g *githubIssues) AddLabelsToIssue(context.Context, string, string, int, []string) ([]*github.Label, *github.Response, error) {
	return nil, nil, nil
}
func (g *githubIssues) CreateComment(context.Context, string, string, int, *github.IssueComment) (*github.IssueComment, *github.Response, error) {
	return nil, nil, nil
}

type githubRepositories struct {
}

func (g *githubRepositories) DownloadContents(ctx context.Context, owner string, name string, file string, opts *github.RepositoryContentGetOptions) (io.ReadCloser, error) {
	dat, _ := ioutil.ReadFile("../.hello.yml")
	return &ClosingBuffer{bytes.NewBufferString(string(dat))}, nil
}

func newClient(httpClient *http.Client) *githubClient {
	return &githubClient{
		Issues:       &githubIssues{},
		Repositories: &githubRepositories{},
	}
}

func TestBot(t *testing.T) {
	b := &Bot{id: 1234, cert: "", ctx: context.Background(), client: newClient(nil)}

	// Test issue with a opened action.
	if err := b.SayHello(issueOpenedRequest); err != nil {
		t.Fatal(err)
	}

	// Test issue with a different action.
	if err := b.SayHello(issueCreatedRequest); err == nil {
		t.Fatal("Only opened actions is allowed")
	}
}

func TestBotGitHubClient(t *testing.T) {
	b := &Bot{id: 1234, cert: "", ctx: context.Background()}

	if err := b.SayHello(issueOpenedRequest); err == nil {
		t.Fatal("Should not work without a GitHub client")
	}
}
