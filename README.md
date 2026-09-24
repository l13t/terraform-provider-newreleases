# Terraform Provider for newreleases.io

Manage [newreleases.io](https://newreleases.io) with Terraform: which projects
are tracked, where their release notifications go (email, Slack, Telegram,
Discord, Matrix, Microsoft Teams, Mattermost, Rocket.Chat, Google Hangouts
Chat, webhooks) and how they are tagged. Data sources expose releases of
tracked projects, so a configuration can pin versions to what newreleases.io
has seen.

Documentation for every resource and data source is in [`docs/`](docs/) and on
the Terraform Registry.

## Usage

```hcl
terraform {
  required_providers {
    newreleases = {
      source = "l13t/newreleases"
    }
  }
}

# Reads the API key from NEWRELEASES_API_KEY.
provider "newreleases" {}

resource "newreleases_tag" "golang" {
  name = "golang"
}

resource "newreleases_project" "go" {
  provider_name       = "github"
  name                = "golang/go"
  email_notification  = "daily"
  exclude_prereleases = true
  tags                = [newreleases_tag.golang.id]
}

data "newreleases_release" "go_latest" {
  id = newreleases_project.go.id
}
```

Create an API key under **Settings → API keys** on newreleases.io and export it:

```sh
export NEWRELEASES_API_KEY="..."
```

Existing projects can be imported by ID or by `<provider>/<name>`:

```sh
terraform import newreleases_project.go github/golang/go
```

## Development

Requirements: Go >= 1.25.8, [go-task](https://taskfile.dev) >= 3, Terraform
>= 1.9 or OpenTofu >= 1.10 (acceptance tests; docs generation needs
Terraform), `gpg` (releases only). Release steps are in
[`.github/RELEASING.md`](.github/RELEASING.md).

`task --list` shows all tasks. The common ones:

| Task | Purpose |
| --- | --- |
| `task build` | Build the provider binary |
| `task check` | fmt, vet, golangci-lint and unit tests |
| `task docs` | Regenerate `docs/` from schemas, `examples/` and `templates/` |
| `task install` | Install into `~/.terraform.d/plugins` as version `0.0.0-dev` |

To run a local build without installing it, point Terraform at the repository
with `dev_overrides` in `~/.terraformrc`, then `task build`:

```hcl
provider_installation {
  dev_overrides {
    "l13t/newreleases" = "/path/to/terraform-provider-newreleases"
  }
  direct {}
}
```

### Acceptance tests

The same acceptance tests run in two modes. `TF_CLI=tofu` switches either of
them from Terraform to OpenTofu.

**Fake API (default).** `task testacc` starts an in-memory fake of the API
(`internal/fakeapi`) inside the test process. No API key, no rate limit, no
changes to any account; CI runs it against Terraform 1.9 and 1.16 and
OpenTofu 1.10 and 1.12.

```sh
task testacc               # Terraform
task testacc TF_CLI=tofu   # OpenTofu
```

**Live API.** `task testacc:live` runs the tests against the real
newreleases.io API. Use it before a release and whenever the fake may have
drifted from the real service.

> **Warning:** live tests create and delete real projects
> (`github/golang/go`, `github/golang/tools`) and tags in the account behind
> the key. Do not run them against an account that already tracks those
> projects. A full run spends roughly 450 of the account's 1000 hourly
> rate-limit units (writes cost 10 each), so run it at most twice per hour.

```sh
NEWRELEASES_API_KEY=... task testacc:live
NEWRELEASES_API_KEY=... task testacc:live TF_CLI=tofu
```

In CI the live run is the manually triggered **Acceptance (live API)**
workflow; it reads the key from the `NEWRELEASES_API_KEY` secret of the
`acceptance-live` environment. See [`.github/RELEASING.md`](.github/RELEASING.md).

## License

[MPL-2.0](LICENSE)
