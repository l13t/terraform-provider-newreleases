package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"

	"github.com/l13t/terraform-provider-newreleases/internal/client"
)

// Acceptance tests track real public repositories because the API validates
// that the project exists upstream; the names cannot be randomized.
const (
	testAccProjectResource = "newreleases_project.test"
	testAccProjectName     = "golang/go"
	testAccProjectAltName  = "golang/tools"
)

func testAccProjectConfigBasic(name string) string {
	return fmt.Sprintf(`
resource "newreleases_project" "test" {
  provider_name = "github"
  name          = %q
}
`, name)
}

func testAccProjectConfigFull() string {
	return fmt.Sprintf(`
resource "newreleases_project" "test" {
  provider_name       = "github"
  name                = %q
  email_notification  = "daily"
  exclude_prereleases = true
  note                = "tracked by terraform"

  exclude_version_regexp = [
    {
      value   = "^go1\\.\\d+beta"
      inverse = false
    },
  ]
}
`, testAccProjectName)
}

func stateCheckProjectExists(t *testing.T, address string) statecheck.StateCheck {
	return stateCheckFunc(func(ctx context.Context, state *tfjson.State) error {
		id, err := stateResourceID(state, address)
		if err != nil {
			return err
		}
		if _, err := testAccClient(t).Projects.GetByID(ctx, id); err != nil {
			return fmt.Errorf("project %s: %w", id, err)
		}
		return nil
	})
}

func stateCheckProjectDisappears(t *testing.T, address string) statecheck.StateCheck {
	return stateCheckFunc(func(ctx context.Context, state *tfjson.State) error {
		id, err := stateResourceID(state, address)
		if err != nil {
			return err
		}
		return testAccClient(t).Projects.DeleteByID(ctx, id)
	})
}

func testAccCheckProjectDestroy(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := testAccClient(t)
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "newreleases_project" {
				continue
			}
			_, err := c.Projects.GetByID(context.Background(), rs.Primary.ID)
			if err == nil {
				return fmt.Errorf("project %s still exists", rs.Primary.ID)
			}
			if !client.IsNotFound(err) {
				return err
			}
		}
		return nil
	}
}

func TestAccProject_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckProjectDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccProjectConfigBasic(testAccProjectName),
				ConfigStateChecks: []statecheck.StateCheck{
					stateCheckProjectExists(t, testAccProjectResource),
					statecheck.ExpectKnownValue(testAccProjectResource, tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccProjectResource, tfjsonpath.New("url"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccProjectResource, tfjsonpath.New("email_notification"), knownvalue.StringExact("none")),
					statecheck.ExpectKnownValue(testAccProjectResource, tfjsonpath.New("exclude_prereleases"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue(testAccProjectResource, tfjsonpath.New("webhooks"), knownvalue.SetSizeExact(0)),
				},
			},
			{
				ResourceName:      testAccProjectResource,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      testAccProjectResource,
				ImportState:       true,
				ImportStateId:     "github/" + testAccProjectName,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccProject_update(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckProjectDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccProjectConfigBasic(testAccProjectName),
			},
			{
				Config: testAccProjectConfigFull(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccProjectResource, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccProjectResource, tfjsonpath.New("email_notification"), knownvalue.StringExact("daily")),
					statecheck.ExpectKnownValue(testAccProjectResource, tfjsonpath.New("exclude_prereleases"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue(testAccProjectResource, tfjsonpath.New("note"), knownvalue.StringExact("tracked by terraform")),
					statecheck.ExpectKnownValue(testAccProjectResource, tfjsonpath.New("exclude_version_regexp"), knownvalue.SetExact([]knownvalue.Check{
						knownvalue.ObjectExact(map[string]knownvalue.Check{
							"value":   knownvalue.StringExact(`^go1\.\d+beta`),
							"inverse": knownvalue.Bool(false),
						}),
					})),
				},
			},
			{
				// Removing the attributes must reset them through their
				// defaults rather than leave the remote values untouched.
				Config: testAccProjectConfigBasic(testAccProjectName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccProjectResource, tfjsonpath.New("email_notification"), knownvalue.StringExact("none")),
					statecheck.ExpectKnownValue(testAccProjectResource, tfjsonpath.New("note"), knownvalue.StringExact("")),
					statecheck.ExpectKnownValue(testAccProjectResource, tfjsonpath.New("exclude_version_regexp"), knownvalue.SetSizeExact(0)),
					statecheck.ExpectKnownValue(testAccProjectResource, tfjsonpath.New("exclude_prereleases"), knownvalue.Bool(false)),
				},
			},
		},
	})
}

func TestAccProject_requiresReplace(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckProjectDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccProjectConfigBasic(testAccProjectName),
			},
			{
				Config: testAccProjectConfigBasic(testAccProjectAltName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccProjectResource, plancheck.ResourceActionReplace),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccProjectResource, tfjsonpath.New("name"), knownvalue.StringExact(testAccProjectAltName)),
				},
			},
		},
	})
}

func TestAccProject_disappears(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckProjectDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccProjectConfigBasic(testAccProjectName),
				ConfigStateChecks: []statecheck.StateCheck{
					stateCheckProjectDisappears(t, testAccProjectResource),
				},
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccProject_withTag(t *testing.T) {
	tagName := "tf-acc-" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckProjectDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "newreleases_tag" "test" {
  name = %q
}

resource "newreleases_project" "test" {
  provider_name = "github"
  name          = %q
  tags          = [newreleases_tag.test.id]
}
`, tagName, testAccProjectName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccProjectResource, tfjsonpath.New("tags"), knownvalue.SetSizeExact(1)),
					statecheck.CompareValueCollection(
						testAccProjectResource, []tfjsonpath.Path{tfjsonpath.New("tags")},
						"newreleases_tag.test", tfjsonpath.New("id"),
						compare.ValuesSame(),
					),
				},
			},
			{
				// Removing the last tag must clear it remotely: the API treats
				// a missing list as "leave unchanged".
				Config: fmt.Sprintf(`
resource "newreleases_tag" "test" {
  name = %q
}

resource "newreleases_project" "test" {
  provider_name = "github"
  name          = %q
}
`, tagName, testAccProjectName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccProjectResource, tfjsonpath.New("tags"), knownvalue.SetSizeExact(0)),
					stateCheckFunc(func(ctx context.Context, state *tfjson.State) error {
						id, err := stateResourceID(state, testAccProjectResource)
						if err != nil {
							return err
						}
						p, err := testAccClient(t).Projects.GetByID(ctx, id)
						if err != nil {
							return err
						}
						if len(p.TagIDs) != 0 {
							return fmt.Errorf("project %s still has tags %v", id, p.TagIDs)
						}
						return nil
					}),
				},
			},
		},
	})
}

// TestAccValidation_* tests fail during validation, before any API call.
func TestAccValidation_projectInvalidEmailNotification(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "newreleases_project" "test" {
  provider_name      = "github"
  name               = %q
  email_notification = "monthly"
}
`, testAccProjectName),
				ExpectError: regexp.MustCompile(`Invalid Attribute Value Match`),
			},
		},
	})
}
