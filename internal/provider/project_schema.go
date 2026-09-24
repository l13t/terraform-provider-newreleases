package provider

import (
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Attribute descriptions shared by the newreleases_project resource and the
// project data sources, so documentation stays identical across them.
const (
	descProjectID              = "Project ID assigned by newreleases.io."
	descProjectProviderName    = "Upstream provider hosting the project, for example `github`, `gitlab`, `pypi`, `npm` or `dockerhub`. See the `newreleases_providers` data source for the full list."
	descProjectName            = "Project name at the upstream provider, for example `golang/go`."
	descProjectURL             = "URL of the project at the upstream provider."
	descProjectEmail           = "Email notification schedule: `none`, `instant`, `hourly`, `daily`, `weekly` or `default` (inherit the account setting)."
	descProjectSlack           = "IDs of Slack channels to notify. See the `newreleases_slack_channels` data source."
	descProjectTelegram        = "IDs of Telegram chats to notify. See the `newreleases_telegram_chats` data source."
	descProjectDiscord         = "IDs of Discord channels to notify. See the `newreleases_discord_channels` data source."
	descProjectHangouts        = "IDs of Google Hangouts Chat webhooks to notify. See the `newreleases_hangouts_chat_webhooks` data source."
	descProjectTeams           = "IDs of Microsoft Teams webhooks to notify. See the `newreleases_microsoft_teams_webhooks` data source."
	descProjectMattermost      = "IDs of Mattermost webhooks to notify. See the `newreleases_mattermost_webhooks` data source."
	descProjectRocketchat      = "IDs of Rocket.Chat webhooks to notify. See the `newreleases_rocketchat_webhooks` data source."
	descProjectMatrix          = "IDs of Matrix rooms to notify. See the `newreleases_matrix_rooms` data source."
	descProjectWebhooks        = "IDs of generic webhooks to notify. See the `newreleases_webhooks` data source."
	descProjectTags            = "IDs of tags assigned to the project. See the `newreleases_tag` resource."
	descProjectExclusions      = "Regular expressions filtering which versions are tracked."
	descProjectExclusionValue  = "Regular expression matched against the version."
	descProjectExclusionInvert = "When true, the regular expression is an inclusion filter: only matching versions are tracked."
	descProjectPrereleases     = "Do not notify about pre-release versions."
	descProjectUpdated         = "Do not notify about updates of already published releases."
	descProjectNote            = "Free-form note attached to the project."
)

// projectDataSourceAttributes returns the computed project attributes for data
// sources. When lookup is true, id, provider_name and name are also optional
// inputs selecting the project.
func projectDataSourceAttributes(lookup bool) map[string]dschema.Attribute {
	strSet := func(desc string) dschema.SetAttribute {
		return dschema.SetAttribute{ElementType: types.StringType, Computed: true, MarkdownDescription: desc}
	}
	return map[string]dschema.Attribute{
		"id":                       dschema.StringAttribute{Optional: lookup, Computed: true, MarkdownDescription: descProjectID},
		"provider_name":            dschema.StringAttribute{Optional: lookup, Computed: true, MarkdownDescription: descProjectProviderName},
		"name":                     dschema.StringAttribute{Optional: lookup, Computed: true, MarkdownDescription: descProjectName},
		"url":                      dschema.StringAttribute{Computed: true, MarkdownDescription: descProjectURL},
		"email_notification":       dschema.StringAttribute{Computed: true, MarkdownDescription: descProjectEmail},
		"slack_channels":           strSet(descProjectSlack),
		"telegram_chats":           strSet(descProjectTelegram),
		"discord_channels":         strSet(descProjectDiscord),
		"hangouts_chat_webhooks":   strSet(descProjectHangouts),
		"microsoft_teams_webhooks": strSet(descProjectTeams),
		"mattermost_webhooks":      strSet(descProjectMattermost),
		"rocketchat_webhooks":      strSet(descProjectRocketchat),
		"matrix_rooms":             strSet(descProjectMatrix),
		"webhooks":                 strSet(descProjectWebhooks),
		"tags":                     strSet(descProjectTags),
		"exclude_version_regexp": dschema.SetNestedAttribute{
			Computed:            true,
			MarkdownDescription: descProjectExclusions,
			NestedObject: dschema.NestedAttributeObject{Attributes: map[string]dschema.Attribute{
				"value":   dschema.StringAttribute{Computed: true, MarkdownDescription: descProjectExclusionValue},
				"inverse": dschema.BoolAttribute{Computed: true, MarkdownDescription: descProjectExclusionInvert},
			}},
		},
		"exclude_prereleases": dschema.BoolAttribute{Computed: true, MarkdownDescription: descProjectPrereleases},
		"exclude_updated":     dschema.BoolAttribute{Computed: true, MarkdownDescription: descProjectUpdated},
		"note":                dschema.StringAttribute{Computed: true, MarkdownDescription: descProjectNote},
	}
}
