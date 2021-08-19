package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/go-github/github"

	"github.com/mattermost/mattermost-plugin-apps/apps"
	"github.com/mattermost/mattermost-plugin-apps/utils/httputils"
	"github.com/pkg/errors"
)

// go:embed server/dylan.jpg
var iconData []byte

func getManifest(siteURL string) apps.Manifest {
	var manifest = apps.Manifest{
		AppID:       "dylan",
		PluginID:    "dylan",
		DisplayName: "Dylan Testing App",
		HomepageURL: "https://github.com/mickmister/mattermost-app-dylan",
		AppType:     apps.AppTypePlugin,
		Icon:        "dylan.jpg",
		RequestedPermissions: apps.Permissions{
			apps.PermissionActAsBot,
		},
		RequestedLocations: apps.Locations{
			apps.LocationCommand,
		},
	}

	return manifest
}

func (p *Plugin) handleManifest(w http.ResponseWriter, r *http.Request) {
	s := p.API.GetConfig().ServiceSettings.SiteURL
	manifest := getManifest(*s)
	httputils.WriteJSON(w, manifest)
}

func (p *Plugin) handleStatic(w http.ResponseWriter, r *http.Request) {
	ptof("Serving icon")
	bundlePath, err := p.API.GetBundlePath()
	if err != nil {
		p.API.LogError("Error getting bundle path", "err", err.Error())
		w.Write([]byte(err.Error()))
		return
	}

	iconBytes, err := ioutil.ReadFile(filepath.Join(bundlePath, "assets", "dylan.jpg"))
	if err != nil {
		p.API.LogError("Error reading static icon", "err", err.Error())
		w.Write([]byte(err.Error()))
		return
	}

	w.Header().Set("Content-Type", "image/jpg")
	w.Write(iconBytes)
}

var register = &BindingRegister{}

var commands = &BindingEntry{
	binding: &apps.Binding{
		Location: apps.LocationCommand,
	},
}
var _ = register.AddEntry(commands)

var dylanCommand = &BindingEntry{
	binding: &apps.Binding{
		Location:    "dylan",
		Label:       "dylan",
		Description: "Build a plugin from a PR",
		Call:        apps.NewCall("/commands/dylan"),
		Form: &apps.Form{
			Fields: []*apps.Field{
				{
					Name:        "pr",
					Label:       "pr",
					Description: "Full URL to a plugin repo's PR",
				},
			},
		},
	},
	handler: func(c *apps.CallRequest, callType string, p *Plugin) *apps.CallResponse {
		if callType == string(apps.CallTypeSubmit) && c.Values["pr"] != nil {
			return handleSubmit(c, p)
		}

		return handleForm(c, p)
	},
}
var _ = commands.AddEntry(dylanCommand)

func handleForm(c *apps.CallRequest, p *Plugin) *apps.CallResponse {
	name := ""
	repo, _ := c.Values["repo"].(map[string]interface{})
	if repo != nil {
		name, _ = repo["value"].(string)
	}

	prs, err := getPRs(p.getGitHubClient(), name, c.Query)
	if err != nil {
		return apps.NewErrorCallResponse(err)
	}

	repos, err := getRepoNames(p.getGitHubClient())
	if err != nil {
		return apps.NewErrorCallResponse(err)
	}

	return &apps.CallResponse{
		Type: apps.CallResponseTypeForm,
		Form: &apps.Form{
			Title: "Build plugin PR for this server",
			Fields: []*apps.Field{
				{
					Name:                "repo",
					ModalLabel:          "Repo",
					Type:                apps.FieldTypeStaticSelect,
					SelectStaticOptions: repos,
					SelectRefresh:       true,
					Value:               c.Values["repo"],
					IsRequired:          true,
				},
				{
					Name:                "pr",
					ModalLabel:          "Pull Request",
					Type:                apps.FieldTypeStaticSelect,
					SelectStaticOptions: prs,
					Value:               c.Values["pr"],
					IsRequired:          true,
				},
			},
		},
	}
}

func handleSubmit(c *apps.CallRequest, p *Plugin) *apps.CallResponse {
	prURL := ""
	switch v := c.Values["pr"].(type) {
	case string:
		prURL = v
	case map[string]interface{}:
		prURL, _ = v["value"].(string)
	default:
		return apps.NewErrorCallResponse(errors.Errorf("invalid form of pr value %T", c.Values["pr"]))
	}

	u, _ := url.Parse(prURL)
	parts := strings.Split(u.Path, "/")
	org, repo, prNum := parts[1], parts[2], parts[4]

	prNumInt, err := strconv.Atoi(prNum)
	if err != nil {
		return apps.NewErrorCallResponse(err)
	}

	gh := github.NewClient(nil)

	p.ephemeral(c, "Getting pull request information")

	pr, _, err := gh.PullRequests.Get(context.Background(), org, repo, prNumInt)
	if err != nil {
		return apps.NewErrorCallResponse(err)
	}

	branch := pr.Head

	p.ephemeral(c, "Getting CI check information")

	checkName := "ci"
	checks, _, err := gh.Checks.ListCheckRunsForRef(context.Background(), org, repo, *branch.SHA, &github.ListCheckRunsOptions{CheckName: &checkName})
	if err != nil {
		return apps.NewErrorCallResponse(err)
	}

	run := checks.CheckRuns[0]

	workflowData := map[string]string{}
	err = json.Unmarshal([]byte(*run.ExternalID), &workflowData)
	if err != nil {
		return apps.NewErrorCallResponse(err)
	}

	p.ephemeral(c, "Getting artifacts URL")

	workflowID := workflowData["workflow-id"]
	artifactURL, err := getArtifactsForJob(workflowID, "plugin-ci/build")

	if err != nil {
		return apps.NewErrorCallResponse(err)
	}

	p.ephemeral(c, "Downloading plugin artifact and installing plugin")

	m, err := p.Helpers.InstallPluginFromURL(artifactURL, true)
	if err != nil {
		return apps.NewErrorCallResponse(err)
	}

	p.ephemeral(c, "Enabling plugin")

	appErr := p.API.EnablePlugin(m.Id)
	if appErr != nil {
		return apps.NewErrorCallResponse(appErr)
	}

	return &apps.CallResponse{
		Markdown: fmt.Sprintf("Built plugin and deployed this server for PR %s", prURL),
	}
}
