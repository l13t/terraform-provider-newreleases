package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"newreleases.io/newreleases"
)

func emptyProjectModel() projectModel {
	m := projectModel{
		EmailNotification:    types.StringValue("none"),
		ExcludeVersionRegexp: emptyExclusionSet(),
		ExcludePrereleases:   types.BoolValue(false),
		ExcludeUpdated:       types.BoolValue(false),
		Note:                 types.StringValue(""),
	}
	for _, f := range m.optionStringSets(&newreleases.ProjectOptions{}) {
		*f.model = emptyStringSet()
	}
	return m
}

func requireNoDiags(t *testing.T, diags diag.Diagnostics) {
	t.Helper()
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestApplyProjectToModel_normalizesEmptyEmailNotification(t *testing.T) {
	var m projectModel
	var diags diag.Diagnostics
	applyProjectToModel(context.Background(), &newreleases.Project{ID: "p1", EmailNotification: ""}, &m, &diags)
	requireNoDiags(t, diags)

	if got := m.EmailNotification.ValueString(); got != "none" {
		t.Errorf("email_notification = %q, want %q", got, "none")
	}
}

func TestApplyProjectToModel_preservesDefaultEmailNotification(t *testing.T) {
	m := projectModel{EmailNotification: types.StringValue("default")}
	var diags diag.Diagnostics
	applyProjectToModel(context.Background(), &newreleases.Project{ID: "p1", EmailNotification: newreleases.EmailNotificationDaily}, &m, &diags)
	requireNoDiags(t, diags)

	if got := m.EmailNotification.ValueString(); got != "default" {
		t.Errorf("email_notification = %q, want %q", got, "default")
	}
}

func TestApplyProjectToModel_nilSlicesBecomeEmptySets(t *testing.T) {
	var m projectModel
	var diags diag.Diagnostics
	applyProjectToModel(context.Background(), &newreleases.Project{ID: "p1"}, &m, &diags)
	requireNoDiags(t, diags)

	sets := map[string]types.Set{"exclude_version_regexp": m.ExcludeVersionRegexp}
	for i, f := range m.optionStringSets(&newreleases.ProjectOptions{}) {
		sets[fmt.Sprintf("string set %d", i)] = *f.model
	}
	if len(sets) != 11 {
		t.Fatalf("checked %d sets, want 11", len(sets))
	}
	for name, s := range sets {
		if s.IsNull() || s.IsUnknown() {
			t.Errorf("set %s is null/unknown, want known empty set", name)
			continue
		}
		if n := len(s.Elements()); n != 0 {
			t.Errorf("set %s has %d elements, want 0", name, n)
		}
	}
}

func TestProjectOptionsFromModel_sendsInitializedSlices(t *testing.T) {
	var diags diag.Diagnostics
	o := projectOptionsFromModel(context.Background(), emptyProjectModel(), &diags)
	requireNoDiags(t, diags)

	slices := map[string][]string{
		"slack_channels":           o.SlackIDs,
		"telegram_chats":           o.TelegramChatIDs,
		"discord_channels":         o.DiscordIDs,
		"hangouts_chat_webhooks":   o.HangoutsChatWebhookIDs,
		"microsoft_teams_webhooks": o.MSTeamsWebhookIDs,
		"mattermost_webhooks":      o.MattermostWebhookIDs,
		"rocketchat_webhooks":      o.RocketchatWebhookIDs,
		"matrix_rooms":             o.MatrixRoomIDs,
		"webhooks":                 o.WebhookIDs,
		"tags":                     o.TagIDs,
	}
	for name, s := range slices {
		if s == nil {
			t.Errorf("%s is a nil slice; the API would treat it as \"leave unchanged\"", name)
		}
	}
	if o.Exclusions == nil {
		t.Error("exclude_version_regexp is a nil slice")
	}
	if o.EmailNotification == nil || o.ExcludePrereleases == nil || o.ExcludeUpdated == nil || o.Note == nil {
		t.Error("scalar options must all be set")
	}
}

func TestProjectOptionsFromModel_roundTripsExclusions(t *testing.T) {
	m := emptyProjectModel()
	elem := func(value string, inverse bool) attr.Value {
		return types.ObjectValueMust(exclusionObjectType().AttrTypes, map[string]attr.Value{
			"value":   types.StringValue(value),
			"inverse": types.BoolValue(inverse),
		})
	}
	m.ExcludeVersionRegexp = types.SetValueMust(exclusionObjectType(), []attr.Value{
		elem(`^v2\.`, true),
		elem(`beta`, false),
	})

	var diags diag.Diagnostics
	o := projectOptionsFromModel(context.Background(), m, &diags)
	requireNoDiags(t, diags)

	want := map[string]bool{`^v2\.`: true, `beta`: false}
	if len(o.Exclusions) != len(want) {
		t.Fatalf("got %d exclusions, want %d", len(o.Exclusions), len(want))
	}
	for _, e := range o.Exclusions {
		inverse, ok := want[e.Value]
		if !ok {
			t.Errorf("unexpected exclusion %q", e.Value)
			continue
		}
		if e.Inverse != inverse {
			t.Errorf("exclusion %q inverse = %v, want %v", e.Value, e.Inverse, inverse)
		}
	}
}
