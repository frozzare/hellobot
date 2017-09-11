package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
)

// Bot represents the bot.
type Bot struct {
	id      int
	cert    string
	config  *Config
	client  *github.Client
	payload *Payload
}

// NewBot creates a new bot instance.
func NewBot(id int, cert string) *Bot {
	return &Bot{id: id, cert: cert}
}

// validatePayload validates the payload from github.
func (b *Bot) validatePayload() error {
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

// number returns the issue or pull request number.
func (b *Bot) number() int {
	number := b.payload.Issue.Number
	if b.payload.IsPullRequest() {
		number = b.payload.PullRequest.Number
	}
	return number
}

// Item returns the message item (issue or pull request)
func (b *Bot) item() Item {
	if b.payload.IsPullRequest() {
		return b.config.PullRequest
	}

	return b.config.Issue
}

// createClient creates a new GitHub client.
func (b *Bot) createClient() (*github.Client, error) {
	tr := http.DefaultTransport
	itr, err := ghinstallation.NewKeyFromFile(tr, b.id, b.payload.Installation.ID, b.cert)
	if err != nil {
		return nil, err
	}
	return github.NewClient(&http.Client{Transport: itr}), nil
}

// downloadConfig downloads the bot configuration from GitHub.
func (b *Bot) downloadConfig() (*Config, error) {
	buf, err := b.client.Repositories.DownloadContents(context.Background(), b.payload.Repository.Owner.Login, b.payload.Repository.Name, ".hello.yml", &github.RepositoryContentGetOptions{})
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

	if err := json.NewDecoder(r.Body).Decode(&b.payload); err != nil {
		return errors.Wrap(err, "unmarshal payload")
	}

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
	item := b.item()
	if item.Disabled {
		return errors.New("Item disabled")
	}

	number := b.number()

	// Create GitHub comment.
	_, _, err = b.client.Issues.CreateComment(
		context.Background(),
		b.payload.Repository.Owner.Login,
		b.payload.Repository.Name,
		number,
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
			context.Background(),
			b.payload.Repository.Owner.Login,
			b.payload.Repository.Name,
			number,
			item.Labels,
		)

		return errors.Wrap(err, "github add labels to issue")
	}

	return err
}
