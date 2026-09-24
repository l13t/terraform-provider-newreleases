package provider

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"newreleases.io/newreleases"

	"github.com/l13t/terraform-provider-newreleases/internal/client"
)

var (
	_ resource.Resource                = &projectResource{}
	_ resource.ResourceWithConfigure   = &projectResource{}
	_ resource.ResourceWithImportState = &projectResource{}
)

// NewProjectResource returns the newreleases_project resource.
func NewProjectResource() resource.Resource {
	return &projectResource{}
}

type projectResource struct {
	client *newreleases.Client
}

func (r *projectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *projectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	strSet := func(desc string) schema.SetAttribute {
		return schema.SetAttribute{
			ElementType:         types.StringType,
			Optional:            true,
			Computed:            true,
			Default:             setdefault.StaticValue(emptyStringSet()),
			MarkdownDescription: desc,
		}
	}
	required := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{
			Required:            true,
			MarkdownDescription: desc,
			PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
		}
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Tracks a project on newreleases.io and configures where its release notifications are delivered.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: descProjectID,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"provider_name": required(descProjectProviderName + " Changing this forces a new project."),
			"name":          required(descProjectName + " Changing this forces a new project."),
			"url": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: descProjectURL,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"email_notification": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(emailNotificationNone),
				MarkdownDescription: descProjectEmail + " Defaults to `none`.",
				Validators: []validator.String{
					stringvalidator.OneOf("none", "instant", "hourly", "daily", "weekly", "default"),
				},
			},
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
			"exclude_version_regexp": schema.SetNestedAttribute{
				Optional:            true,
				Computed:            true,
				Default:             setdefault.StaticValue(emptyExclusionSet()),
				MarkdownDescription: descProjectExclusions,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"value": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: descProjectExclusionValue,
						Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
					},
					"inverse": schema.BoolAttribute{
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
						MarkdownDescription: descProjectExclusionInvert,
					},
				}},
			},
			"exclude_prereleases": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: descProjectPrereleases,
			},
			"exclude_updated": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: descProjectUpdated,
			},
			"note": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: descProjectNote,
			},
		},
	}
}

func (r *projectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := clientFromProviderData(req.ProviderData, &resp.Diagnostics); c != nil {
		r.client = c
	}
}

func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := projectOptionsFromModel(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	providerName, name := plan.ProviderName.ValueString(), plan.Name.ValueString()
	p, err := r.client.Projects.Add(ctx, providerName, name, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error creating newreleases project",
			client.Detail(fmt.Sprintf("creating project (%s/%s)", providerName, name), err))
		return
	}

	// Only id and url are taken from the response: every optional field of
	// newreleases.Project is omitempty, so the response does not round-trip
	// the request and normalizing it here would yield "inconsistent result
	// after apply". Real drift is caught by the next refresh.
	plan.ID = types.StringValue(p.ID)
	plan.URL = types.StringValue(p.URL)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	p, err := r.client.Projects.GetByID(ctx, id)
	if client.IsNotFound(err) {
		tflog.Warn(ctx, "newreleases project not found, removing from state", map[string]any{"id": id})
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading newreleases project",
			client.Detail(fmt.Sprintf("reading project %s", id), err))
		return
	}

	applyProjectToModel(ctx, p, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *projectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state projectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// provider_name and name force replacement, so only options can differ.
	planOpts := projectOptionsFromModel(ctx, plan, &resp.Diagnostics)
	stateOpts := projectOptionsFromModel(ctx, state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if !reflect.DeepEqual(planOpts, stateOpts) {
		id := plan.ID.ValueString()
		if _, err := r.client.Projects.UpdateByID(ctx, id, planOpts); err != nil {
			resp.Diagnostics.AddError("Error updating newreleases project",
				client.Detail(fmt.Sprintf("updating project %s", id), err))
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	err := r.client.Projects.DeleteByID(ctx, id)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting newreleases project",
			client.Detail(fmt.Sprintf("deleting project %s", id), err))
	}
}

// ImportState accepts either a project ID or "<provider>/<name>". Project IDs
// never contain a slash, so the two forms are unambiguous.
func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	providerName, name, found := strings.Cut(req.ID, "/")
	if !found {
		resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
		return
	}

	p, err := r.client.Projects.GetByName(ctx, providerName, name)
	if client.IsNotFound(err) {
		resp.Diagnostics.AddError("Project not found", fmt.Sprintf("no project %q at provider %q", name, providerName))
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error importing newreleases project",
			client.Detail(fmt.Sprintf("reading project %s", req.ID), err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), p.ID)...)
}
