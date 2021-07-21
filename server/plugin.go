package main

import (
	"sync"

	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-plugin-apps/apps"
	"github.com/mattermost/mattermost-plugin-apps/apps/mmclient"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration
}

func (p *Plugin) OnActivate() error {
	appsPluginClient := mmclient.NewAppsPluginAPIClientFromPluginAPI(&pluginapi.NewClient(p.API, p.Driver).Plugin)
	s := p.API.GetConfig().ServiceSettings.SiteURL
	manifest := getManifest(*s)

	err := appsPluginClient.InstallApp(manifest)
	if err != nil {
		return err
	}

	return nil
}

func (p *Plugin) ephemeral(c *apps.CallRequest, message string) {
	post := &model.Post{
		UserId:    c.Context.BotUserID,
		Message:   message,
		ChannelId: c.Context.ChannelID,
	}

	p.API.SendEphemeralPost(c.Context.ActingUserID, post)
}

// See https://developers.mattermost.com/extend/plugins/server/reference/
