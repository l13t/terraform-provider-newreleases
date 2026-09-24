# Every provider supported by newreleases.io.
data "newreleases_providers" "all" {}

# Only providers with at least one tracked project.
data "newreleases_providers" "used" {
  added = true
}
