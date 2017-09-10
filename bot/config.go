package bot

// Config represents `.hello.yml` file.
type Config struct {
	Ignore struct {
		Users  []string `yaml:"users"`
		Labels []string `yaml:"labels"`
	} `yaml:"ignore"`
	Issue       Item `yaml:"issue"`
	PullRequest Item `yaml:"pull_request"`
}

type Item struct {
	Disabled bool     `yaml:"disabled"`
	Labels   []string `yaml:"labels"`
	Message  string   `yaml:"message"`
}
