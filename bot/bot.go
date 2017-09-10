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
	id     int
	cert   string
	config *Config
}

// NewBot creates a new bot instance.
func NewBot(id int, cert string) *Bot {
	return &Bot{id: id, cert: cert}
}

// validatePayload validates the payload from github.
func (b *Bot) validatePayload(payload *Payload) error {
	for _, user := range b.config.Ignore.Users {
		if strings.ToLower(user) == strings.ToLower(payload.Sender.Login) {
			return fmt.Errorf("User with login %s should be ignored", user)
		}
	}

	for _, label := range payload.Issue.Labels {
		for _, name := range b.config.Ignore.Labels {
			if strings.ToLower(name) == strings.ToLower(label.Name) {
				return fmt.Errorf("Issue or pull request with label %s should be ignored", name)
			}
		}
	}

	return nil
}

func (b *Bot) item(p *Payload) Item {
	if p.IsPullRequest() {
		return b.config.PullRequest
	}

	return b.config.Issue
}

// SayHello will take a http request, decode the request body and write a hello comment.
func (b *Bot) SayHello(r *http.Request) error {
	var err error
	var payload *Payload

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return errors.Wrap(err, "unmarshal payload")
	}

	if payload.Action != "opened" {
		return errors.New("Only opened action is handled")
	}

	if payload.Repository.Private {
		return errors.New("Only public repository can be used")
	}

	tr := http.DefaultTransport
	itr, err := ghinstallation.NewKeyFromFile(tr, b.id, payload.Installation.ID, b.cert)
	if err != nil {
		return err
	}
	client := github.NewClient(&http.Client{Transport: itr})

	buf, err := client.Repositories.DownloadContents(context.Background(), payload.Repository.Owner.Login, payload.Repository.Name, ".hello.yml", &github.RepositoryContentGetOptions{})
	if err != nil {
		return errors.Wrap(err, "downloading github file")
	}

	data, err := ioutil.ReadAll(buf)
	if err != nil {
		return errors.Wrap(err, "reading github file")
	}

	var config *Config

	if err := yaml.Unmarshal(data, &config); err != nil {
		return errors.Wrap(err, "unmarshal yaml")
	}

	b.config = config

	if err := b.validatePayload(payload); err != nil {
		return errors.Wrap(err, "validate payload")
	}

	if config.Issue.Disabled {
		return errors.New("Issue comment is disabled")
	}

	item := b.item(payload)
	if item.Disabled {
		return errors.New("Item disabled")
	}

	number := payload.Issue.Number
	if payload.IsPullRequest() {
		number = payload.PullRequest.Number
	}

	body := strings.Replace(item.Message, "@{author}", payload.Sender.Login, -1)

	_, _, err = client.Issues.CreateComment(
		context.Background(),
		payload.Repository.Owner.Login,
		payload.Repository.Name,
		number,
		&github.IssueComment{
			Body: github.String(body),
		},
	)

	if err := errors.Wrap(err, "github create comment"); err != nil {
		return err
	}

	if len(item.Labels) > 0 {
		_, _, err = client.Issues.AddLabelsToIssue(
			context.Background(),
			payload.Repository.Owner.Login,
			payload.Repository.Name,
			number,
			item.Labels,
		)

		return errors.Wrap(err, "github add labels to issue")
	}

	return err
}
