package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"newreleases.io/newreleases"

	"github.com/l13t/terraform-provider-newreleases/internal/client"
)

var (
	_ datasource.DataSource                     = &releaseDataSource{}
	_ datasource.DataSourceWithConfigure        = &releaseDataSource{}
	_ datasource.DataSourceWithConfigValidators = &releaseDataSource{}
)

// NewReleaseDataSource returns the newreleases_release data source.
func NewReleaseDataSource() datasource.DataSource {
	return &releaseDataSource{}
}

type releaseDataSource struct {
	client *newreleases.Client
}

type releaseDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	ProviderName types.String `tfsdk:"provider_name"`
	Name         types.String `tfsdk:"name"`
	Version      types.String `tfsdk:"version"`
	Date         types.String `tfsdk:"date"`
	CVE          []string     `tfsdk:"cve"`
	IsPrerelease types.Bool   `tfsdk:"is_prerelease"`
	IsUpdated    types.Bool   `tfsdk:"is_updated"`
	IsExcluded   types.Bool   `tfsdk:"is_excluded"`
	HasNote      types.Bool   `tfsdk:"has_note"`
	NoteTitle    types.String `tfsdk:"note_title"`
	NoteMessage  types.String `tfsdk:"note_message"`
	NoteURL      types.String `tfsdk:"note_url"`
}

func (d *releaseDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_release"
}

func (d *releaseDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a single release of a tracked project. Omit `version` to read the latest non-excluded release — useful for pinning a version elsewhere in the configuration.",
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "ID of the tracked project. Conflicts with `provider_name`."},
			"provider_name": schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Upstream provider of the tracked project. Requires `name`."},
			"name":          schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Name of the tracked project. Requires `provider_name`."},
			"version":       schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Release version. When omitted, the latest non-excluded release is read and this attribute is set to its version."},
			"date":          schema.StringAttribute{Computed: true, MarkdownDescription: "Release date in RFC 3339 format."},
			"cve": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "CVE identifiers referenced by the release.",
			},
			"is_prerelease": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the release is a pre-release."},
			"is_updated":    schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the release was updated after publication."},
			"is_excluded":   schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the release is excluded by the project's filters."},
			"has_note":      schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the release has a release note."},
			"note_title":    schema.StringAttribute{Computed: true, MarkdownDescription: "Release note title; null when the release has no note."},
			"note_message":  schema.StringAttribute{Computed: true, MarkdownDescription: "Release note body (HTML); null when the release has no note."},
			"note_url":      schema.StringAttribute{Computed: true, MarkdownDescription: "Release note URL; null when the release has no note."},
		},
	}
}

func (d *releaseDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return projectLookupValidators()
}

func (d *releaseDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c := clientFromProviderData(req.ProviderData, &resp.Diagnostics); c != nil {
		d.client = c
	}
}

func (d *releaseDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m releaseDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	byID := !m.ID.IsNull()
	id, providerName, name := m.ID.ValueString(), m.ProviderName.ValueString(), m.Name.ValueString()
	ref := id
	if !byID {
		ref = providerName + "/" + name
	}

	var (
		release *newreleases.Release
		err     error
	)
	switch {
	case m.Version.IsNull() && byID:
		release, err = d.client.Releases.GetLatestByProjectID(ctx, id)
	case m.Version.IsNull():
		release, err = d.client.Releases.GetLatestByProjectName(ctx, providerName, name)
	case byID:
		release, err = d.client.Releases.GetByProjectID(ctx, id, m.Version.ValueString())
	default:
		release, err = d.client.Releases.GetByProjectName(ctx, providerName, name, m.Version.ValueString())
	}
	if client.IsNotFound(err) {
		resp.Diagnostics.AddError("Release not found", fmt.Sprintf("no release %q of project %q", m.Version.ValueString(), ref))
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading newreleases release", client.Detail(fmt.Sprintf("reading release of project %s", ref), err))
		return
	}

	m.Version = types.StringValue(release.Version)
	m.Date = types.StringValue(release.Date.Format(time.RFC3339))
	m.CVE = release.CVE
	if m.CVE == nil {
		m.CVE = []string{}
	}
	m.IsPrerelease = types.BoolValue(release.IsPrerelease)
	m.IsUpdated = types.BoolValue(release.IsUpdated)
	m.IsExcluded = types.BoolValue(release.IsExcluded)
	m.HasNote = types.BoolValue(release.HasNote)
	m.NoteTitle, m.NoteMessage, m.NoteURL = types.StringNull(), types.StringNull(), types.StringNull()

	if release.HasNote {
		var note *newreleases.ReleaseNote
		if byID {
			note, err = d.client.Releases.GetNoteByProjectID(ctx, id, release.Version)
		} else {
			note, err = d.client.Releases.GetNoteByProjectName(ctx, providerName, name, release.Version)
		}
		switch {
		case client.IsNotFound(err):
			// The note can be published after the release; treat it as absent.
		case err != nil:
			resp.Diagnostics.AddError("Error reading newreleases release note",
				client.Detail(fmt.Sprintf("reading note of release %s of project %s", release.Version, ref), err))
			return
		default:
			m.NoteTitle = types.StringValue(note.Title)
			m.NoteMessage = types.StringValue(note.Message)
			m.NoteURL = types.StringValue(note.URL)
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}
