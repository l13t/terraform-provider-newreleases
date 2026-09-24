package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"newreleases.io/newreleases"

	"github.com/l13t/terraform-provider-newreleases/internal/client"
)

var (
	_ datasource.DataSource                     = &projectDataSource{}
	_ datasource.DataSourceWithConfigure        = &projectDataSource{}
	_ datasource.DataSourceWithConfigValidators = &projectDataSource{}
)

// NewProjectDataSource returns the newreleases_project data source.
func NewProjectDataSource() datasource.DataSource {
	return &projectDataSource{}
}

type projectDataSource struct {
	client *newreleases.Client
}

// projectLookupValidators require either id or the provider_name + name pair.
func projectLookupValidators() []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.Conflicting(path.MatchRoot("id"), path.MatchRoot("provider_name")),
		datasourcevalidator.RequiredTogether(path.MatchRoot("provider_name"), path.MatchRoot("name")),
		datasourcevalidator.AtLeastOneOf(path.MatchRoot("id"), path.MatchRoot("provider_name")),
	}
}

func (d *projectDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *projectDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a tracked project by `id` or by `provider_name` and `name`.",
		Attributes:          projectDataSourceAttributes(true),
	}
}

func (d *projectDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return projectLookupValidators()
}

func (d *projectDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c := clientFromProviderData(req.ProviderData, &resp.Diagnostics); c != nil {
		d.client = c
	}
}

func (d *projectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m projectModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var (
		p   *newreleases.Project
		err error
		ref string
	)
	if !m.ID.IsNull() {
		ref = m.ID.ValueString()
		p, err = d.client.Projects.GetByID(ctx, ref)
	} else {
		ref = m.ProviderName.ValueString() + "/" + m.Name.ValueString()
		p, err = d.client.Projects.GetByName(ctx, m.ProviderName.ValueString(), m.Name.ValueString())
	}
	if client.IsNotFound(err) {
		resp.Diagnostics.AddError("Project not found", fmt.Sprintf("no tracked project %q", ref))
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading newreleases project", client.Detail(fmt.Sprintf("reading project %s", ref), err))
		return
	}

	applyProjectToModel(ctx, p, &m, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}
