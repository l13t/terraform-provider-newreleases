# All tracked GitHub projects, sorted by name.
data "newreleases_projects" "github" {
  provider_name = "github"
  order         = "name"
}

# Projects with a given tag.
data "newreleases_projects" "infra" {
  tag_id = newreleases_tag.infra.id
}

# Projects matching a search query.
data "newreleases_projects" "hashicorp" {
  search_query = "hashicorp"
}
