package bot

// Payload struct of GitHub webhooks for issues and pull request.
type Payload struct {
	Action string `json:"action"`
	Issue  struct {
		Number int `json:"number"`
		Labels []struct {
			Name string `json:"string"`
		} `json:"labels"`
	} `json:"issue"`
	PullRequest struct {
		Number int `json:"number"`
	} `json:"pull_request"`
	Repository struct {
		DefaultBranch string `json:"default_branch"`
		Name          string `json:"name"`
		Owner         struct {
			Login string `json:"login"`
		} `json:"owner"`
		Private bool `json:"private"`
	} `json:"repository"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
	Installation struct {
		ID int `json:"id"`
	} `json:"installation"`
}

// IsPullRequest returns true when the payload it's a pull request payload.
func (p *Payload) IsPullRequest() bool {
	return p.PullRequest.Number > 0 && p.Issue.Number == 0
}
