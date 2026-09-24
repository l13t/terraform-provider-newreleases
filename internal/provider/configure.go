package provider

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"newreleases.io/newreleases"
)

// clientFromProviderData extracts the API client from ProviderData. It returns
// nil when the provider is not configured yet (validation phase), which is not
// an error.
func clientFromProviderData(providerData any, diags *diag.Diagnostics) *newreleases.Client {
	if providerData == nil {
		return nil
	}
	c, ok := providerData.(*newreleases.Client)
	if !ok {
		diags.AddError(
			"Unexpected Provider Data Type",
			fmt.Sprintf("Expected *newreleases.Client, got: %T. Please report this issue to the provider developers.", providerData),
		)
		return nil
	}
	return c
}
