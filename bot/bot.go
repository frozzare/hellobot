package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
)

type githubIssuesService interface {
	AddLabelsToIssue(context.Context, string, string, int, []string) ([]*github.Label, *github.Response, error)
	CreateComment(context.Context, string, string, int, *github.IssueComment) (*github.IssueComment, *github.Response, error)
}

type githubRepositoriesService interface {
	DownloadContents(context.Context, string, string, string, *github.RepositoryContentGetOptions) (io.ReadCloser, error)
}

type githubClient struct {
	Issues       githubIssuesService
	Repositories githubRepositoriesService
}

// Bot represents the bot.
type Bot struct {
	id      int
	cert    string
	config  *Config
	client  *githubClient
	payload *Payload
	ctx     context.Context
}

// NewBot creates a new bot instance.
func NewBot(id int, cert string) *Bot {
	return &Bot{id: id, cert: cert, ctx: context.Background()}
}

// validatePayload validates the payload from github.
func (b *Bot) validatePayload() error {
	if b.payload == nil {
		return errors.New("No payload exists")
	}

	if b.config == nil {
		return errors.New("No config exists")
	}

	for _, user := range b.config.Ignore.Users {
		if strings.ToLower(user) == strings.ToLower(b.payload.Sender.Login) {
			return fmt.Errorf("User with login %s should be ignored", user)
		}
	}

	for _, label := range b.payload.Issue.Labels {
		for _, name := range b.config.Ignore.Labels {
			if strings.ToLower(name) == strings.ToLower(label.Name) {
				return fmt.Errorf("Issue or pull request with label %s should be ignored", name)
			}
		}
	}

	return nil
}

// Item returns the message item (issue or pull request)
func (b *Bot) item() (Item, error) {
	if b.config == nil {
		return Item{}, errors.New("No config exists")
	}

	if b.payload.IsPullRequest() {
		return b.config.PullRequest, nil
	}

	return b.config.Issue, nil
}

// createClient creates a new GitHub client.
func (b *Bot) createClient() (*githubClient, error) {
	if b.payload == nil {
		return nil, errors.New("No payload exists")
	}

	if b.client != nil {
		return b.client, nil
	}

	tr := http.DefaultTransport
	itr, err := ghinstallation.NewKeyFromFile(tr, b.id, b.payload.Installation.ID, b.cert)
	if err != nil {
		return nil, err
	}

	client := github.NewClient(&http.Client{Transport: itr})

	return &githubClient{
		Issues:       client.Issues,
		Repositories: client.Repositories,
	}, nil
}

// downloadConfig downloads the bot configuration from GitHub.
func (b *Bot) downloadConfig() (*Config, error) {
	if b.payload == nil {
		return nil, errors.New("No payload exists")
	}

	if b.client == nil {
		return nil, errors.New("No GitHub client")
	}

	buf, err := b.client.Repositories.DownloadContents(b.ctx, b.payload.Repository.Owner.Login, b.payload.Repository.Name, ".hello.yml", &github.RepositoryContentGetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "downloading github file")
	}

	data, err := ioutil.ReadAll(buf)
	if err != nil {
		return nil, errors.Wrap(err, "reading github file")
	}

	var config *Config

	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, errors.Wrap(err, "unmarshal yaml")
	}

	return config, nil
}

// SayHello will take a http request, decode the request body and write a hello comment.
func (b *Bot) SayHello(r *http.Request) error {
	var err error
	var payload *Payload

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return errors.Wrap(err, "unmarshal payload")
	}

	b.payload = payload

	// Only issues or pull requests with "opened" action is allowed.
	if b.payload.Action != "opened" {
		return errors.New("Only opened action is handled")
	}

	// Only open GitHub projects is allowed.
	if b.payload.Repository.Private {
		return errors.New("Only public repository can be used")
	}

	// Create GitHub client.
	b.client, err = b.createClient()
	if err != nil {
		return err
	}

	// Download config from GitHub.
	b.config, err = b.downloadConfig()
	if err != nil {
		return err
	}

	// Validate payload with config values.
	if err := b.validatePayload(); err != nil {
		return errors.Wrap(err, "validate payload")
	}

	// Get message item (issue or pull requelst).
	item, err := b.item()
	if err != nil {
		return err
	}
	if item.Disabled {
		return errors.New("Item disabled")
	}

	if b.client == nil {
		return errors.New("No GitHub client")
	}

	fmt.Println("number", b.payload.Number)

	// Create GitHub comment.
	_, _, err = b.client.Issues.CreateComment(
		b.ctx,
		b.payload.Repository.Owner.Login,
		b.payload.Repository.Name,
		b.payload.Number,
		&github.IssueComment{
			Body: github.String(strings.Replace(item.Message, "@{author}", b.payload.Sender.Login, -1)),
		},
	)

	if err := errors.Wrap(err, "github create comment"); err != nil {
		return err
	}

	// Add labels to GitHub issue if any.
	if len(item.Labels) > 0 {
		_, _, err = b.client.Issues.AddLabelsToIssue(
			b.ctx,
			b.payload.Repository.Owner.Login,
			b.payload.Repository.Name,
			b.payload.Number,
			item.Labels,
		)

		return errors.Wrap(err, "github add labels to issue")
	}

	return err
}
