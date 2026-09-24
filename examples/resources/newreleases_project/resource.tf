# Minimal: track releases without notifications.
resource "newreleases_project" "terraform" {
  provider_name = "github"
  name          = "hashicorp/terraform"
}

# Full: daily email digest, a Slack channel, a tag and version filters.
data "newreleases_slack_channels" "all" {}

resource "newreleases_tag" "golang" {
  name = "golang"
}

resource "newreleases_project" "go" {
  provider_name       = "github"
  name                = "golang/go"
  email_notification  = "daily"
  slack_channels      = [for c in data.newreleases_slack_channels.all.channels : c.id if c.channel == "releases"]
  tags                = [newreleases_tag.golang.id]
  exclude_prereleases = true
  note                = "Managed by Terraform"

  exclude_version_regexp = [
    {
      value = "^weekly\\."
    },
    {
      # Only track stable go1.x releases.
      value   = "^go1\\.\\d+(\\.\\d+)?$"
      inverse = true
    },
  ]
}
