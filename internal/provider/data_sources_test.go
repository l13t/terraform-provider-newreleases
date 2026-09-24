package provider

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"testing"

	tfjson "github.com/hashicorp/terraform-json"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func testAccDataSourceTest(t *testing.T, config string, checks ...statecheck.StateCheck) {
	t.Helper()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckProjectDestroy(t),
		Steps: []resource.TestStep{
			{Config: config, ConfigStateChecks: checks},
		},
	})
}

func TestAccProjectDataSource_byID(t *testing.T) {
	testAccDataSourceTest(t, testAccProjectConfigBasic(testAccProjectName)+`
data "newreleases_project" "test" {
  id = newreleases_project.test.id
}
`,
		statecheck.CompareValuePairs(
			"data.newreleases_project.test", tfjsonpath.New("id"),
			testAccProjectResource, tfjsonpath.New("id"),
			compare.ValuesSame(),
		),
		statecheck.ExpectKnownValue("data.newreleases_project.test", tfjsonpath.New("name"), knownvalue.StringExact(testAccProjectName)),
		statecheck.ExpectKnownValue("data.newreleases_project.test", tfjsonpath.New("email_notification"), knownvalue.StringExact("none")),
	)
}

func TestAccProjectDataSource_byName(t *testing.T) {
	testAccDataSourceTest(t, testAccProjectConfigBasic(testAccProjectName)+`
data "newreleases_project" "test" {
  provider_name = newreleases_project.test.provider_name
  name          = newreleases_project.test.name
}
`,
		statecheck.CompareValuePairs(
			"data.newreleases_project.test", tfjsonpath.New("id"),
			testAccProjectResource, tfjsonpath.New("id"),
			compare.ValuesSame(),
		),
		statecheck.ExpectKnownValue("data.newreleases_project.test", tfjsonpath.New("tags"), knownvalue.SetSizeExact(0)),
	)
}

func TestAccValidation_projectDataSourceConflictingArgs(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "newreleases_project" "test" {
  id            = "abc"
  provider_name = "github"
  name          = "golang/go"
}
`,
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
		},
	})
}

func TestAccProjectsDataSource_basic(t *testing.T) {
	testAccDataSourceTest(t, testAccProjectConfigBasic(testAccProjectName)+`
data "newreleases_projects" "test" {
  provider_name = "github"
  order         = "name"
  depends_on    = [newreleases_project.test]
}

data "newreleases_projects" "search" {
  search_query = "golang"
  depends_on   = [newreleases_project.test]
}
`,
		statecheck.CompareValueCollection(
			"data.newreleases_projects.test", []tfjsonpath.Path{tfjsonpath.New("projects"), tfjsonpath.New("id")},
			testAccProjectResource, tfjsonpath.New("id"),
			compare.ValuesSame(),
		),
		statecheck.CompareValueCollection(
			"data.newreleases_projects.search", []tfjsonpath.Path{tfjsonpath.New("projects"), tfjsonpath.New("id")},
			testAccProjectResource, tfjsonpath.New("id"),
			compare.ValuesSame(),
		),
	)
}

func TestAccProvidersDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "newreleases_providers" "test" {}`,
				ConfigStateChecks: []statecheck.StateCheck{
					stateCheckFunc(func(_ context.Context, state *tfjson.State) error {
						r, err := stateResourceAtAddress(state, "data.newreleases_providers.test")
						if err != nil {
							return err
						}
						names, _ := r.AttributeValues["names"].([]any)
						if !slices.Contains(names, any("github")) {
							return fmt.Errorf("names %v does not contain github", names)
						}
						return nil
					}),
				},
			},
		},
	})
}

func TestAccTagsDataSource_basic(t *testing.T) {
	name := testAccTagName()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckTagDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccTagConfig(name) + `
data "newreleases_tags" "test" {
  depends_on = [newreleases_tag.test]
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.CompareValueCollection(
						"data.newreleases_tags.test", []tfjsonpath.Path{tfjsonpath.New("tags"), tfjsonpath.New("id")},
						testAccTagResource, tfjsonpath.New("id"),
						compare.ValuesSame(),
					),
				},
			},
		},
	})
}

func TestAccReleaseDataSource_latest(t *testing.T) {
	testAccDataSourceTest(t, testAccProjectConfigBasic(testAccProjectName)+`
data "newreleases_release" "test" {
  id = newreleases_project.test.id
}
`,
		statecheck.ExpectKnownValue("data.newreleases_release.test", tfjsonpath.New("version"), knownvalue.NotNull()),
		statecheck.ExpectKnownValue("data.newreleases_release.test", tfjsonpath.New("date"), knownvalue.NotNull()),
		statecheck.ExpectKnownValue("data.newreleases_release.test", tfjsonpath.New("cve"), knownvalue.NotNull()),
	)
}

func TestAccReleaseDataSource_specificVersion(t *testing.T) {
	testAccDataSourceTest(t, testAccProjectConfigBasic(testAccProjectName)+`
data "newreleases_release" "test" {
  provider_name = newreleases_project.test.provider_name
  name          = newreleases_project.test.name
  version       = "go1.21.0"
}
`,
		statecheck.ExpectKnownValue("data.newreleases_release.test", tfjsonpath.New("version"), knownvalue.StringExact("go1.21.0")),
		statecheck.ExpectKnownValue("data.newreleases_release.test", tfjsonpath.New("is_prerelease"), knownvalue.Bool(false)),
	)
}

// The account may have no channels connected, so these only prove that each
// endpoint is reachable and yields a collection.
func TestAccNotificationTargetDataSources_basic(t *testing.T) {
	targets := map[string]string{
		"slack_channels":           "channels",
		"telegram_chats":           "chats",
		"matrix_rooms":             "rooms",
		"discord_channels":         "channels",
		"webhooks":                 "webhooks",
		"hangouts_chat_webhooks":   "webhooks",
		"microsoft_teams_webhooks": "webhooks",
		"mattermost_webhooks":      "webhooks",
		"rocketchat_webhooks":      "webhooks",
	}
	for suffix, collection := range targets {
		t.Run(suffix, func(t *testing.T) {
			address := "data.newreleases_" + suffix + ".test"
			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config: fmt.Sprintf(`data "newreleases_%s" "test" {}`, suffix),
						ConfigStateChecks: []statecheck.StateCheck{
							statecheck.ExpectKnownValue(address, tfjsonpath.New(collection), knownvalue.NotNull()),
						},
					},
				},
			})
		})
	}
}
