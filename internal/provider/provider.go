// Package provider implements the newreleases Terraform provider.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/l13t/terraform-provider-newreleases/internal/client"
)

var _ provider.Provider = &newreleasesProvider{}

// New returns the provider factory consumed by main.go.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &newreleasesProvider{version: version}
	}
}

type newreleasesProvider struct {
	version string
}

type newreleasesProviderModel struct {
	APIKey  types.String `tfsdk:"api_key"`
	BaseURL types.String `tfsdk:"base_url"`
}

func (p *newreleasesProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "newreleases"
	resp.Version = p.version
}

func (p *newreleasesProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages tracked projects and tags on [newreleases.io](https://newreleases.io).",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "API key for newreleases.io. Generate one under Settings → API keys. May also be set via the `NEWRELEASES_API_KEY` environment variable.",
			},
			"base_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Base URL of the API. Defaults to `https://api.newreleases.io/`. May also be set via the `NEWRELEASES_BASE_URL` environment variable. Intended for testing against a stub server.",
			},
		},
	}
}

func (p *newreleasesProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config newreleasesProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Values wired to other resources' outputs are unknown during planning;
	// treating them as empty would silently mis-authenticate.
	if config.APIKey.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Unknown API key",
			"api_key depends on a value known only after apply. Set a static value or use the NEWRELEASES_API_KEY environment variable.",
		)
	}
	if config.BaseURL.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("base_url"),
			"Unknown base URL",
			"base_url depends on a value known only after apply. Set a static value or use the NEWRELEASES_BASE_URL environment variable.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// Explicit configuration wins; environment variables are the fallback.
	apiKey := config.APIKey.ValueString()
	if apiKey == "" {
		apiKey = os.Getenv("NEWRELEASES_API_KEY")
	}
	baseURL := config.BaseURL.ValueString()
	if baseURL == "" {
		baseURL = os.Getenv("NEWRELEASES_BASE_URL")
	}

	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing API key",
			"Set api_key in the provider block or export NEWRELEASES_API_KEY.",
		)
		return
	}

	c, err := client.New(client.Config{APIKey: apiKey, BaseURL: baseURL})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create newreleases API client", err.Error())
		return
	}
	resp.ResourceData = c
	resp.DataSourceData = c
}

func (p *newreleasesProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewProjectResource,
		NewTagResource,
	}
}

func (p *newreleasesProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewProjectDataSource,
		NewProjectsDataSource,
		NewProvidersDataSource,
		NewTagsDataSource,
		NewReleaseDataSource,
		NewSlackChannelsDataSource,
		NewTelegramChatsDataSource,
		NewMatrixRoomsDataSource,
		NewWebhooksDataSource,
		NewDiscordChannelsDataSource,
		NewHangoutsChatWebhooksDataSource,
		NewMicrosoftTeamsWebhooksDataSource,
		NewMattermostWebhooksDataSource,
		NewRocketchatWebhooksDataSource,
	}
}
