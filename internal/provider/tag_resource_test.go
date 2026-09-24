package provider

import (
	"context"
	"fmt"
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"

	"github.com/l13t/terraform-provider-newreleases/internal/client"
)

const testAccTagResource = "newreleases_tag.test"

func testAccTagName() string {
	return "tf-acc-" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
}

func testAccTagConfig(name string) string {
	return fmt.Sprintf(`
resource "newreleases_tag" "test" {
  name = %q
}
`, name)
}

func testAccCheckTagDestroy(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := testAccClient(t)
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "newreleases_tag" {
				continue
			}
			_, err := c.Tags.Get(context.Background(), rs.Primary.ID)
			if err == nil {
				return fmt.Errorf("tag %s still exists", rs.Primary.ID)
			}
			if !client.IsNotFound(err) {
				return err
			}
		}
		return nil
	}
}

func TestAccTag_basic(t *testing.T) {
	name := testAccTagName()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckTagDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccTagConfig(name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccTagResource, tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(testAccTagResource, tfjsonpath.New("name"), knownvalue.StringExact(name)),
				},
			},
			{
				ResourceName:      testAccTagResource,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccTag_update(t *testing.T) {
	name, renamed := testAccTagName(), testAccTagName()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckTagDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccTagConfig(name),
			},
			{
				Config: testAccTagConfig(renamed),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(testAccTagResource, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(testAccTagResource, tfjsonpath.New("name"), knownvalue.StringExact(renamed)),
				},
			},
		},
	})
}

func TestAccTag_disappears(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckTagDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccTagConfig(testAccTagName()),
				ConfigStateChecks: []statecheck.StateCheck{
					stateCheckFunc(func(ctx context.Context, state *tfjson.State) error {
						id, err := stateResourceID(state, testAccTagResource)
						if err != nil {
							return err
						}
						return testAccClient(t).Tags.Delete(ctx, id)
					}),
				},
				ExpectNonEmptyPlan: true,
			},
		},
	})
}
