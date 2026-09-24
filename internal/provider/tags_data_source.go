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
	_ datasource.DataSource              = &tagsDataSource{}
	_ datasource.DataSourceWithConfigure = &tagsDataSource{}
)

// NewTagsDataSource returns the newreleases_tags data source.
func NewTagsDataSource() datasource.DataSource {
	return &tagsDataSource{}
}

type tagsDataSource struct {
	client *newreleases.Client
}

type tagsDataSourceModel struct {
	Tags []tagModel `tfsdk:"tags"`
}

func (d *tagsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tags"
}

func (d *tagsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all tags in the account.",
		Attributes: map[string]schema.Attribute{
			"tags": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Tags in the account.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":   schema.StringAttribute{Computed: true, MarkdownDescription: "Tag ID."},
					"name": schema.StringAttribute{Computed: true, MarkdownDescription: "Tag name."},
				}},
			},
		},
	}
}

func (d *tagsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c := clientFromProviderData(req.ProviderData, &resp.Diagnostics); c != nil {
		d.client = c
	}
}

func (d *tagsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	tags, err := d.client.Tags.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing newreleases tags", client.Detail("listing tags", err))
		return
	}
	m := tagsDataSourceModel{Tags: make([]tagModel, 0, len(tags))}
	for _, t := range tags {
		m.Tags = append(m.Tags, tagModel{ID: types.StringValue(t.ID), Name: types.StringValue(t.Name)})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}
