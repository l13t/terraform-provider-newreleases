# Latest non-excluded release of a tracked project.
data "newreleases_release" "go_latest" {
  provider_name = "github"
  name          = "golang/go"
}

# A specific release, including its release note.
data "newreleases_release" "go1_21" {
  id      = newreleases_project.go.id
  version = "go1.21.0"
}
