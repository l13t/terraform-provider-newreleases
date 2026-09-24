# Look up a tracked project by provider and name...
data "newreleases_project" "go" {
  provider_name = "github"
  name          = "golang/go"
}

# ...or by ID.
data "newreleases_project" "by_id" {
  id = "pf4w494lbjsd3ydp5hnf4gsptw"
}
