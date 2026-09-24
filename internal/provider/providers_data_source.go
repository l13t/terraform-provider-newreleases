package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"newreleases.io/newreleases"

	"github.com/l13t/terraform-provider-newreleases/internal/client"
)

var (
	_ datasource.DataSource              = &providersDataSource{}
	_ datasource.DataSourceWithConfigure = &providersDataSource{}
)

// NewProvidersDataSource returns the newreleases_providers data source.
func NewProvidersDataSource() datasource.DataSource {
	return &providersDataSource{}
}

type providersDataSource struct {
	client *newreleases.Client
}

type providersDataSourceModel struct {
	Added types.Bool `tfsdk:"added"`
	Names []string   `tfsdk:"names"`
}

func (d *providersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_providers"
}

func (d *providersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists upstream providers supported by newreleases.io, the valid values of `provider_name`.",
		Attributes: map[string]schema.Attribute{
			"added": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "When true, only list providers that have at least one tracked project in the account.",
			},
			"names": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "Provider names.",
			},
		},
	}
}

func (d *providersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c := clientFromProviderData(req.ProviderData, &resp.Diagnostics); c != nil {
		d.client = c
	}
}

func (d *providersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m providersDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var (
		names []string
		err   error
	)
	if m.Added.ValueBool() {
		names, err = d.client.Providers.ListAdded(ctx)
	} else {
		names, err = d.client.Providers.List(ctx)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error listing newreleases providers", client.Detail("listing providers", err))
		return
	}
	if names == nil {
		names = []string{}
	}
	m.Names = names
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}
