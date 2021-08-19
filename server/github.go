package main

import (
	"context"
	"sort"
	"strings"

	"github.com/google/go-github/github"
	"github.com/mattermost/mattermost-plugin-apps/apps"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

func (p *Plugin) getGitHubClient() *github.Client {
	token := p.getConfiguration().GitHubAccessToken
	if token == "" {
		return github.NewClient(nil)
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)

	return github.NewClient(tc)
}

func getPRs(client *github.Client, repo, query string) ([]apps.SelectOption, error) {
	if repo == "" {
		return []apps.SelectOption{}, nil
	}

	fullQuery := "is:pr is:open repo:mattermost/" + repo + " " + query
	prs, _, err := client.Search.Issues(context.Background(), fullQuery, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch PRs")
	}

	opts := []apps.SelectOption{}

	for _, pr := range prs.Issues {
		opts = append(opts, apps.SelectOption{
			Label: pr.GetTitle(),
			Value: *pr.HTMLURL,
		})
	}

	return opts, nil
}

func getRepoNames(client *github.Client) ([]apps.SelectOption, error) {
	repos, _, err := client.Search.Repositories(context.Background(), "org:mattermost repo:mattermost-plugin-", &github.SearchOptions{ListOptions: github.ListOptions{PerPage: 100}})
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch repos")
	}

	opts := []apps.SelectOption{}
	names := []string{}

	for _, repo := range repos.Repositories {
		key := *repo.Name
		if strings.HasPrefix(key, "mattermost-plugin") {
			names = append(names, key)
		}
	}

	sort.Strings(names)

	for _, name := range names {
		opts = append(opts, apps.SelectOption{
			Label: name,
			Value: name,
		})
	}

	return opts, nil
}
