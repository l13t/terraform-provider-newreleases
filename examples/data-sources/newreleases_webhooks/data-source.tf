data "newreleases_webhooks" "all" {}

resource "newreleases_project" "terraform" {
  provider_name = "github"
  name          = "hashicorp/terraform"
  webhooks      = [for w in data.newreleases_webhooks.all.webhooks : w.id if w.name == "deploy-bot"]
}
