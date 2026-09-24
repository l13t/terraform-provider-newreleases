package provider

import (
	"context"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"newreleases.io/newreleases"
)

const emailNotificationNone = "none"

// projectModel is the Terraform representation of a newreleases project,
// shared by the newreleases_project resource and data sources.
type projectModel struct {
	ID                     types.String `tfsdk:"id"`
	ProviderName           types.String `tfsdk:"provider_name"`
	Name                   types.String `tfsdk:"name"`
	URL                    types.String `tfsdk:"url"`
	EmailNotification      types.String `tfsdk:"email_notification"`
	SlackChannels          types.Set    `tfsdk:"slack_channels"`
	TelegramChats          types.Set    `tfsdk:"telegram_chats"`
	DiscordChannels        types.Set    `tfsdk:"discord_channels"`
	HangoutsChatWebhooks   types.Set    `tfsdk:"hangouts_chat_webhooks"`
	MicrosoftTeamsWebhooks types.Set    `tfsdk:"microsoft_teams_webhooks"`
	MattermostWebhooks     types.Set    `tfsdk:"mattermost_webhooks"`
	RocketchatWebhooks     types.Set    `tfsdk:"rocketchat_webhooks"`
	MatrixRooms            types.Set    `tfsdk:"matrix_rooms"`
	Webhooks               types.Set    `tfsdk:"webhooks"`
	Tags                   types.Set    `tfsdk:"tags"`
	ExcludeVersionRegexp   types.Set    `tfsdk:"exclude_version_regexp"`
	ExcludePrereleases     types.Bool   `tfsdk:"exclude_prereleases"`
	ExcludeUpdated         types.Bool   `tfsdk:"exclude_updated"`
	Note                   types.String `tfsdk:"note"`
}

type exclusionModel struct {
	Value   types.String `tfsdk:"value"`
	Inverse types.Bool   `tfsdk:"inverse"`
}

// stringSetField pairs a model set attribute with the matching API slice.
type stringSetField struct {
	model *types.Set
	api   *[]string
}

func (m *projectModel) projectStringSets(p *newreleases.Project) []stringSetField {
	return []stringSetField{
		{&m.SlackChannels, &p.SlackIDs},
		{&m.TelegramChats, &p.TelegramChatIDs},
		{&m.DiscordChannels, &p.DiscordIDs},
		{&m.HangoutsChatWebhooks, &p.HangoutsChatWebhookIDs},
		{&m.MicrosoftTeamsWebhooks, &p.MSTeamsWebhookIDs},
		{&m.MattermostWebhooks, &p.MattermostWebhookIDs},
		{&m.RocketchatWebhooks, &p.RocketchatWebhookIDs},
		{&m.MatrixRooms, &p.MatrixRoomIDs},
		{&m.Webhooks, &p.WebhookIDs},
		{&m.Tags, &p.TagIDs},
	}
}

func (m *projectModel) optionStringSets(o *newreleases.ProjectOptions) []stringSetField {
	return []stringSetField{
		{&m.SlackChannels, &o.SlackIDs},
		{&m.TelegramChats, &o.TelegramChatIDs},
		{&m.DiscordChannels, &o.DiscordIDs},
		{&m.HangoutsChatWebhooks, &o.HangoutsChatWebhookIDs},
		{&m.MicrosoftTeamsWebhooks, &o.MSTeamsWebhookIDs},
		{&m.MattermostWebhooks, &o.MattermostWebhookIDs},
		{&m.RocketchatWebhooks, &o.RocketchatWebhookIDs},
		{&m.MatrixRooms, &o.MatrixRoomIDs},
		{&m.Webhooks, &o.WebhookIDs},
		{&m.Tags, &o.TagIDs},
	}
}

// exclusionObjectType is the element type of the exclude_version_regexp set.
func exclusionObjectType() types.ObjectType {
	return types.ObjectType{AttrTypes: map[string]attr.Type{
		"value":   types.StringType,
		"inverse": types.BoolType,
	}}
}

// emptyStringSet and emptyExclusionSet are the static defaults of the
// resource's collection attributes.
func emptyStringSet() types.Set { return types.SetValueMust(types.StringType, []attr.Value{}) }

func emptyExclusionSet() types.Set {
	return types.SetValueMust(exclusionObjectType(), []attr.Value{})
}

// projectOptionsFromModel builds a complete ProjectOptions: every pointer is
// non-nil and every slice is initialized, because the API treats a nil slice
// as "leave unchanged" and removing the last channel would be impossible.
// Slices are sorted so equal sets produce equal options.
func projectOptionsFromModel(ctx context.Context, m projectModel, diags *diag.Diagnostics) *newreleases.ProjectOptions {
	email := newreleases.EmailNotification(m.EmailNotification.ValueString())
	if email == "" {
		email = newreleases.EmailNotification(emailNotificationNone)
	}
	o := &newreleases.ProjectOptions{
		EmailNotification:  &email,
		ExcludePrereleases: newreleases.Bool(m.ExcludePrereleases.ValueBool()),
		ExcludeUpdated:     newreleases.Bool(m.ExcludeUpdated.ValueBool()),
		Note:               newreleases.String(m.Note.ValueString()),
	}

	for _, f := range m.optionStringSets(o) {
		ids := []string{}
		if !f.model.IsNull() && !f.model.IsUnknown() {
			diags.Append(f.model.ElementsAs(ctx, &ids, false)...)
		}
		slices.Sort(ids)
		*f.api = ids
	}

	var exclusions []exclusionModel
	if !m.ExcludeVersionRegexp.IsNull() && !m.ExcludeVersionRegexp.IsUnknown() {
		diags.Append(m.ExcludeVersionRegexp.ElementsAs(ctx, &exclusions, false)...)
	}
	o.Exclusions = make([]newreleases.Exclusion, 0, len(exclusions))
	for _, e := range exclusions {
		o.Exclusions = append(o.Exclusions, newreleases.Exclusion{
			Value:   e.Value.ValueString(),
			Inverse: e.Inverse.ValueBool(),
		})
	}
	slices.SortFunc(o.Exclusions, func(a, b newreleases.Exclusion) int {
		if c := strings.Compare(a.Value, b.Value); c != 0 {
			return c
		}
		switch {
		case a.Inverse == b.Inverse:
			return 0
		case a.Inverse:
			return 1
		default:
			return -1
		}
	})
	return o
}

// applyProjectToModel writes an API response into the model. It is used by
// the resource's Read (refresh) and by the data sources. Normalizations:
//   - an empty EmailNotification becomes "none";
//   - when the model already holds "default", EmailNotification is not
//     overwritten (the API resolves "default" to a concrete schedule, which
//     would otherwise produce a perpetual diff);
//   - nil slices become empty sets, not null.
func applyProjectToModel(ctx context.Context, p *newreleases.Project, m *projectModel, diags *diag.Diagnostics) {
	m.ID = types.StringValue(p.ID)
	m.ProviderName = types.StringValue(p.Provider)
	m.Name = types.StringValue(p.Name)
	m.URL = types.StringValue(p.URL)

	if m.EmailNotification.ValueString() != string(newreleases.EmailNotificationDefault) {
		email := string(p.EmailNotification)
		if email == "" {
			email = emailNotificationNone
		}
		m.EmailNotification = types.StringValue(email)
	}

	for _, f := range m.projectStringSets(p) {
		ids := *f.api
		if ids == nil {
			ids = []string{}
		}
		v, d := types.SetValueFrom(ctx, types.StringType, ids)
		diags.Append(d...)
		*f.model = v
	}

	exclusions := make([]exclusionModel, 0, len(p.Exclusions))
	for _, e := range p.Exclusions {
		exclusions = append(exclusions, exclusionModel{
			Value:   types.StringValue(e.Value),
			Inverse: types.BoolValue(e.Inverse),
		})
	}
	v, d := types.SetValueFrom(ctx, exclusionObjectType(), exclusions)
	diags.Append(d...)
	m.ExcludeVersionRegexp = v

	m.ExcludePrereleases = types.BoolValue(p.ExcludePrereleases)
	m.ExcludeUpdated = types.BoolValue(p.ExcludeUpdated)
	m.Note = types.StringValue(p.Note)
}
