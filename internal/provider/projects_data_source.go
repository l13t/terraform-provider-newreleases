package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"newreleases.io/newreleases"

	"github.com/l13t/terraform-provider-newreleases/internal/client"
)

// maxProjectPages stops pagination if the API reports a broken total_pages.
const maxProjectPages = 1000

var (
	_ datasource.DataSource                     = &projectsDataSource{}
	_ datasource.DataSourceWithConfigure        = &projectsDataSource{}
	_ datasource.DataSourceWithConfigValidators = &projectsDataSource{}
)

// NewProjectsDataSource returns the newreleases_projects data source.
func NewProjectsDataSource() datasource.DataSource {
	return &projectsDataSource{}
}

type projectsDataSource struct {
	client *newreleases.Client
}

type projectsDataSourceModel struct {
	ProviderName types.String   `tfsdk:"provider_name"`
	TagID        types.String   `tfsdk:"tag_id"`
	SearchQuery  types.String   `tfsdk:"search_query"`
	Order        types.String   `tfsdk:"order"`
	Reverse      types.Bool     `tfsdk:"reverse"`
	Projects     []projectModel `tfsdk:"projects"`
}

func (d *projectsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_projects"
}

func (d *projectsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists tracked projects, optionally filtered by provider or tag, or matched by a search query.",
		Attributes: map[string]schema.Attribute{
			"provider_name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Only return projects hosted at this upstream provider.",
			},
			"tag_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Only return projects with this tag ID. Conflicts with `search_query`.",
			},
			"search_query": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Return projects whose name matches this query instead of listing all of them. Can be combined with `provider_name` only.",
			},
			"order": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Sort order: `name`, `updated` or `added`. Conflicts with `search_query`.",
				Validators:          []validator.String{stringvalidator.OneOf("name", "updated", "added")},
			},
			"reverse": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Reverse the sort order. Conflicts with `search_query`.",
			},
			"projects": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Matching projects.",
				NestedObject:        schema.NestedAttributeObject{Attributes: projectDataSourceAttributes(false)},
			},
		},
	}
}

func (d *projectsDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	// The API ignores the other list parameters when a query is passed; make
	// that an explicit error instead of silently dropping them.
	q := path.MatchRoot("search_query")
	return []datasource.ConfigValidator{
		datasourcevalidator.Conflicting(q, path.MatchRoot("tag_id")),
		datasourcevalidator.Conflicting(q, path.MatchRoot("order")),
		datasourcevalidator.Conflicting(q, path.MatchRoot("reverse")),
	}
}

func (d *projectsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c := clientFromProviderData(req.ProviderData, &resp.Diagnostics); c != nil {
		d.client = c
	}
}

func (d *projectsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m projectsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projects, ok := d.fetch(ctx, m, resp)
	if !ok {
		return
	}

	m.Projects = make([]projectModel, 0, len(projects))
	for i := range projects {
		var pm projectModel
		applyProjectToModel(ctx, &projects[i], &pm, &resp.Diagnostics)
		m.Projects = append(m.Projects, pm)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

func (d *projectsDataSource) fetch(ctx context.Context, m projectsDataSourceModel, resp *datasource.ReadResponse) ([]newreleases.Project, bool) {
	if !m.SearchQuery.IsNull() {
		projects, err := d.client.Projects.Search(ctx, m.SearchQuery.ValueString(), m.ProviderName.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error searching newreleases projects", client.Detail("searching projects", err))
			return nil, false
		}
		return projects, true
	}

	opts := newreleases.ProjectListOptions{
		Order:    newreleases.ProjectListOrder(m.Order.ValueString()),
		Reverse:  m.Reverse.ValueBool(),
		Provider: m.ProviderName.ValueString(),
		TagID:    m.TagID.ValueString(),
	}
	var all []newreleases.Project
	for page := 1; ; page++ {
		if page > maxProjectPages {
			resp.Diagnostics.AddError("Too many project pages",
				fmt.Sprintf("stopped after %d pages; the API may be reporting an invalid total_pages", maxProjectPages))
			return nil, false
		}
		opts.Page = page
		projects, lastPage, err := d.client.Projects.List(ctx, opts)
		if err != nil {
			resp.Diagnostics.AddError("Error listing newreleases projects", client.Detail(fmt.Sprintf("listing projects page %d", page), err))
			return nil, false
		}
		all = append(all, projects...)
		if page >= lastPage {
			return all, true
		}
	}
}
