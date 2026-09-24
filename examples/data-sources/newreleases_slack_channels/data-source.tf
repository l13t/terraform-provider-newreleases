data "newreleases_slack_channels" "all" {}

# Notify a specific channel by name.
resource "newreleases_project" "terraform" {
  provider_name  = "github"
  name           = "hashicorp/terraform"
  slack_channels = [for c in data.newreleases_slack_channels.all.channels : c.id if c.channel == "releases"]
}
