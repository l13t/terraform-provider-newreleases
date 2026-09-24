package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"newreleases.io/newreleases"

	"github.com/l13t/terraform-provider-newreleases/internal/client"
)

var (
	_ datasource.DataSource              = &notificationTargetsDataSource{}
	_ datasource.DataSourceWithConfigure = &notificationTargetsDataSource{}
)

// targetField is one string attribute of a notification target.
type targetField struct {
	attr string
	desc string
}

// notificationTargetsDataSource lists notification targets (Slack channels,
// webhooks, ...). One type serves nine data sources that differ only in
// naming, the attributes of each target and the API call.
type notificationTargetsDataSource struct {
	client *newreleases.Client

	typeSuffix          string
	collectionAttr      string
	markdownDescription string
	collectionMarkdown  string
	fields              []targetField
	// list returns one row per target, values ordered as fields.
	list func(context.Context, *newreleases.Client) ([][]string, error)
}

var idNameFields = []targetField{
	{"id", "Target ID."},
	{"name", "Target name."},
}

func webhooksDataSource(typeSuffix, label string, list func(context.Context, *newreleases.Client) ([]newreleases.Webhook, error)) datasource.DataSource {
	return &notificationTargetsDataSource{
		typeSuffix:          typeSuffix,
		collectionAttr:      "webhooks",
		markdownDescription: fmt.Sprintf("Lists %s configured in the account. Use their IDs in `newreleases_project.%s`.", label, typeSuffix),
		collectionMarkdown:  fmt.Sprintf("Configured %s.", label),
		fields:              idNameFields,
		list: func(ctx context.Context, c *newreleases.Client) ([][]string, error) {
			webhooks, err := list(ctx, c)
			rows := make([][]string, 0, len(webhooks))
			for _, w := range webhooks {
				rows = append(rows, []string{w.ID, w.Name})
			}
			return rows, err
		},
	}
}

// NewWebhooksDataSource returns the newreleases_webhooks data source.
func NewWebhooksDataSource() datasource.DataSource {
	return webhooksDataSource("webhooks", "generic webhooks", func(ctx context.Context, c *newreleases.Client) ([]newreleases.Webhook, error) {
		return c.Webhooks.List(ctx)
	})
}

// NewHangoutsChatWebhooksDataSource returns the newreleases_hangouts_chat_webhooks data source.
func NewHangoutsChatWebhooksDataSource() datasource.DataSource {
	return webhooksDataSource("hangouts_chat_webhooks", "Google Hangouts Chat webhooks", func(ctx context.Context, c *newreleases.Client) ([]newreleases.Webhook, error) {
		return c.HangoutsChatWebhooks.List(ctx)
	})
}

// NewMicrosoftTeamsWebhooksDataSource returns the newreleases_microsoft_teams_webhooks data source.
func NewMicrosoftTeamsWebhooksDataSource() datasource.DataSource {
	return webhooksDataSource("microsoft_teams_webhooks", "Microsoft Teams webhooks", func(ctx context.Context, c *newreleases.Client) ([]newreleases.Webhook, error) {
		return c.MicrosoftTeamsWebhooks.List(ctx)
	})
}

// NewMattermostWebhooksDataSource returns the newreleases_mattermost_webhooks data source.
func NewMattermostWebhooksDataSource() datasource.DataSource {
	return webhooksDataSource("mattermost_webhooks", "Mattermost webhooks", func(ctx context.Context, c *newreleases.Client) ([]newreleases.Webhook, error) {
		return c.MattermostWebhooks.List(ctx)
	})
}

// NewRocketchatWebhooksDataSource returns the newreleases_rocketchat_webhooks data source.
func NewRocketchatWebhooksDataSource() datasource.DataSource {
	return webhooksDataSource("rocketchat_webhooks", "Rocket.Chat webhooks", func(ctx context.Context, c *newreleases.Client) ([]newreleases.Webhook, error) {
		return c.RocketchatWebhooks.List(ctx)
	})
}

// NewDiscordChannelsDataSource returns the newreleases_discord_channels data source.
func NewDiscordChannelsDataSource() datasource.DataSource {
	return &notificationTargetsDataSource{
		typeSuffix:          "discord_channels",
		collectionAttr:      "channels",
		markdownDescription: "Lists Discord channels connected to the account. Use their IDs in `newreleases_project.discord_channels`.",
		collectionMarkdown:  "Connected Discord channels.",
		fields:              idNameFields,
		list: func(ctx context.Context, c *newreleases.Client) ([][]string, error) {
			channels, err := c.DiscordChannels.List(ctx)
			rows := make([][]string, 0, len(channels))
			for _, ch := range channels {
				rows = append(rows, []string{ch.ID, ch.Name})
			}
			return rows, err
		},
	}
}

// NewSlackChannelsDataSource returns the newreleases_slack_channels data source.
func NewSlackChannelsDataSource() datasource.DataSource {
	return &notificationTargetsDataSource{
		typeSuffix:          "slack_channels",
		collectionAttr:      "channels",
		markdownDescription: "Lists Slack channels connected to the account. Use their IDs in `newreleases_project.slack_channels`.",
		collectionMarkdown:  "Connected Slack channels.",
		fields: []targetField{
			{"id", "Channel ID."},
			{"channel", "Slack channel name."},
			{"team_name", "Slack workspace name."},
		},
		list: func(ctx context.Context, c *newreleases.Client) ([][]string, error) {
			channels, err := c.SlackChannels.List(ctx)
			rows := make([][]string, 0, len(channels))
			for _, ch := range channels {
				rows = append(rows, []string{ch.ID, ch.Channel, ch.TeamName})
			}
			return rows, err
		},
	}
}

// NewTelegramChatsDataSource returns the newreleases_telegram_chats data source.
func NewTelegramChatsDataSource() datasource.DataSource {
	return &notificationTargetsDataSource{
		typeSuffix:          "telegram_chats",
		collectionAttr:      "chats",
		markdownDescription: "Lists Telegram chats connected to the account. Use their IDs in `newreleases_project.telegram_chats`.",
		collectionMarkdown:  "Connected Telegram chats.",
		fields: []targetField{
			{"id", "Chat ID."},
			{"type", "Chat type, for example `private` or `group`."},
			{"name", "Chat name."},
		},
		list: func(ctx context.Context, c *newreleases.Client) ([][]string, error) {
			chats, err := c.TelegramChats.List(ctx)
			rows := make([][]string, 0, len(chats))
			for _, ch := range chats {
				rows = append(rows, []string{ch.ID, ch.Type, ch.Name})
			}
			return rows, err
		},
	}
}

// NewMatrixRoomsDataSource returns the newreleases_matrix_rooms data source.
func NewMatrixRoomsDataSource() datasource.DataSource {
	return &notificationTargetsDataSource{
		typeSuffix:          "matrix_rooms",
		collectionAttr:      "rooms",
		markdownDescription: "Lists Matrix rooms connected to the account. Use their IDs in `newreleases_project.matrix_rooms`.",
		collectionMarkdown:  "Connected Matrix rooms.",
		fields: []targetField{
			{"id", "Room ID."},
			{"name", "Room name."},
			{"homeserver_url", "Homeserver URL."},
			{"internal_room_id", "Matrix room ID on the homeserver."},
		},
		list: func(ctx context.Context, c *newreleases.Client) ([][]string, error) {
			rooms, err := c.MatrixRooms.List(ctx)
			rows := make([][]string, 0, len(rooms))
			for _, r := range rooms {
				rows = append(rows, []string{r.ID, r.Name, r.HomeserverURL, r.InternalRoomID})
			}
			return rows, err
		},
	}
}

func (d *notificationTargetsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + d.typeSuffix
}

func (d *notificationTargetsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := make(map[string]schema.Attribute, len(d.fields))
	for _, f := range d.fields {
		attrs[f.attr] = schema.StringAttribute{Computed: true, MarkdownDescription: f.desc}
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: d.markdownDescription,
		Attributes: map[string]schema.Attribute{
			d.collectionAttr: schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: d.collectionMarkdown,
				NestedObject:        schema.NestedAttributeObject{Attributes: attrs},
			},
		},
	}
}

func (d *notificationTargetsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c := clientFromProviderData(req.ProviderData, &resp.Diagnostics); c != nil {
		d.client = c
	}
}

func (d *notificationTargetsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	rows, err := d.list(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Error listing newreleases "+d.typeSuffix, client.Detail("listing "+d.typeSuffix, err))
		return
	}

	attrTypes := make(map[string]attr.Type, len(d.fields))
	for _, f := range d.fields {
		attrTypes[f.attr] = types.StringType
	}
	elemType := types.ObjectType{AttrTypes: attrTypes}

	elems := make([]attr.Value, 0, len(rows))
	for _, row := range rows {
		values := make(map[string]attr.Value, len(d.fields))
		for i, f := range d.fields {
			values[f.attr] = types.StringValue(row[i])
		}
		obj, diags := types.ObjectValue(attrTypes, values)
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	list, diags := types.ListValue(elemType, elems)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(d.collectionAttr), list)...)
}
