# Releasing

How releases of this provider are produced and published. Steps 1-6 are a
one-time setup; after that a release is just a tag push (see
[Every release](#every-release)).

## How a release is built

Pushing a tag matching `v*` runs `.github/workflows/release.yml`. The
`goreleaser` job imports the GPG signing key from repository secrets and runs
goreleaser (`.goreleaser.yml`), which publishes a GitHub Release containing:

- one `terraform-provider-newreleases_<version>_<os>_<arch>.zip` per platform;
- `terraform-provider-newreleases_<version>_manifest.json` (copy of
  `terraform-registry-manifest.json`);
- `terraform-provider-newreleases_<version>_SHA256SUMS` covering the zips and
  the manifest;
- `terraform-provider-newreleases_<version>_SHA256SUMS.sig`, a binary detached
  GPG signature.

This is the asset layout both the Terraform Registry and the OpenTofu Registry
expect. The registries pick releases up from GitHub; nothing is uploaded to
them directly.

`task release:snapshot` builds the same artifacts locally (unsigned) into
`dist/` without publishing.

## One-time setup

### 1. Signing key

The registries accept **RSA or DSA** keys only; the GnuPG default (ECC,
ed25519) is rejected. Use a dedicated key for releases: its private half ends
up in GitHub secrets.

```sh
gpg --full-generate-key   # kind: "RSA and RSA", 4096 bits
gpg --list-secret-keys --keyid-format long   # note the key ID
```

Note the expiry date. Before it passes, extend it
(`gpg --edit-key <KEY_ID>`, then `expire`) and upload the refreshed public key
to the registries again (steps 2 and 7).

### 2. Public key in the Terraform Registry

1. Sign in to <https://registry.terraform.io> with the GitHub account that owns
   the repository.
2. **User Settings → Signing Keys** (<https://registry.terraform.io/settings/gpg-keys>),
   select the `l13t` namespace and paste the output of:

   ```sh
   gpg --armor --export <KEY_ID>
   ```

The key must be registered before the first release is ingested, otherwise
the signature check fails.

### 3. Repository secrets

```sh
gpg --armor --export-secret-keys <KEY_ID> \
  | gh secret set GPG_PRIVATE_KEY -R l13t/terraform-provider-newreleases
gh secret set PASSPHRASE -R l13t/terraform-provider-newreleases   # prompts for the passphrase
```

Workflow permissions need no change: `release.yml` requests
`contents: write` itself.

For the manually triggered **Acceptance (live API)** workflow, also store an
API key in the `acceptance-live` environment (created on first use; it can
carry protection rules such as required reviewers). Prefer a dedicated
newreleases.io account: the tests create and delete `github/golang/go`,
`github/golang/tools` and tags.

```sh
gh secret set NEWRELEASES_API_KEY --env acceptance-live -R l13t/terraform-provider-newreleases
```

### 4. Push the code

```sh
git remote add origin git@github.com:l13t/terraform-provider-newreleases.git
git push -u origin master
```

Wait for the `Tests` workflow to pass.

### 5. First tag

The tag must be a semantic version prefixed with `v`, and no branch may have
the same name.

```sh
git tag v0.1.0
git push origin v0.1.0
```

Check that the `Release` workflow succeeded and that the GitHub Release lists
the zips, `_SHA256SUMS`, `_SHA256SUMS.sig` and `_manifest.json`.

### 6. Publish to the Terraform Registry

On <https://registry.terraform.io>: **Publish → Provider**, select
`l13t/terraform-provider-newreleases`, accept the terms. The Registry installs
a webhook on `release` events, so later tags are ingested automatically. If a
version does not appear, delete the registry.terraform.io webhook in the
repository settings and press **Resync** on the provider's settings page.

### 7. Publish to the OpenTofu Registry (optional)

OpenTofu has its own registry (`registry.opentofu.org`). Submissions are made
**only through the GitHub issue forms** in `opentofu/registry` (issues created
with `gh` or the API are closed unprocessed):

1. [Submit new Provider](https://github.com/opentofu/registry/issues/new?assignees=&labels=provider%2Csubmission&projects=&template=provider.yml&title=Provider%3A+)
   for `l13t/terraform-provider-newreleases`.
2. [Submit new Provider Signing Key](https://github.com/opentofu/registry/issues/new?assignees=&labels=provider-key%2Csubmission&projects=&template=provider_key.yml&title=Provider+Key%3A+)
   with the same ASCII-armored public key as in step 2.

After approval the registry tracks new GitHub Releases on its own.

### 8. Verify

```hcl
terraform {
  required_providers {
    newreleases = {
      source  = "l13t/newreleases"
      version = "0.1.0"
    }
  }
}
```

`terraform init` (and `tofu init`, once step 7 is approved) must download the
provider and verify its signature.

## Every release

1. Run the acceptance tests against the real API manually, either locally
   (`NEWRELEASES_API_KEY=... task testacc:live`) or via
   **Actions → Acceptance (live API) → Run workflow**. CI only covers the
   fake API; this is what catches changes on the newreleases.io side.
2. Tag and push:

   ```sh
   git tag vX.Y.Z
   git push origin vX.Y.Z
   ```

Never move or re-publish an existing tag: users would get checksum errors.
Fix forward with a new version instead. Pre-releases (`vX.Y.Z-rc1`) are
published but never selected automatically.

## Tool versions

goreleaser is pinned in two places that must stay in sync: `version:` in
`.github/workflows/release.yml` and `release:snapshot` in `Taskfile.yml`.
