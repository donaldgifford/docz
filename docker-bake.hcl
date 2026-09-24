// docker-bake.hcl — multi-arch build pipeline for the repository's images.
//
// One target family per component (DESIGN-0017 §8): `-api` builds docz-api
// from Dockerfile.api. Each family has three targets:
//   - dev-<c>:     local single-arch build, loaded into the Docker daemon
//   - ci-<c>:      multi-arch validation build, cache only, no push
//   - release-<c>: multi-arch build + push (CI only, gated on tag)
//
// Groups keep the component-free spellings working: `default` (a bare
// `docker buildx bake`) and `ci` (CI's docker-build job) build every
// component's dev/ci target.
//
// CI workflow consumes this via docker/bake-action@v6 with the `targets`
// input. The release workflow merges in tag-derived image refs from
// docker/metadata-action's bake-file outputs.

variable "REGISTRY" {
  default = "ghcr.io"
}

// Image name per component. Not IMAGE_NAME: ecr.yml exports an env var of
// that name, and bake reads same-named env vars into variables.
variable "API_IMAGE" {
  default = "donaldgifford/docz-api"
}

variable "VERSION" {
  default = "dev"
}

variable "COMMIT_SHA" {
  default = ""
}

variable "BUILD_DATE" {
  default = ""
}

function "tags" {
  params = [image, version]
  result = version == "dev" ? [
    "${REGISTRY}/${image}:dev",
    ] : [
    "${REGISTRY}/${image}:${version}",
    "${REGISTRY}/${image}:latest",
  ]
}

group "default" {
  targets = ["dev-api"]
}

group "ci" {
  targets = ["ci-api"]
}

// Base target for docz-api.
target "_common_api" {
  dockerfile = "Dockerfile.api"
  context    = "."
  // Build args feed Dockerfile.api's VERSION/COMMIT/DATE ARGs, which the
  // build injects via -ldflags into main.version/commit/date. The bake
  // variables (VERSION/COMMIT_SHA/BUILD_DATE) are set by the publish
  // workflows; without this block every image compiled in version=dev.
  args = {
    VERSION = "${VERSION}"
    COMMIT  = "${COMMIT_SHA}"
    DATE    = "${BUILD_DATE}"
  }
  labels = {
    "org.opencontainers.image.source"      = "https://github.com/donaldgifford/docz"
    "org.opencontainers.image.revision"    = "${COMMIT_SHA}"
    "org.opencontainers.image.created"     = "${BUILD_DATE}"
    "org.opencontainers.image.version"     = "${VERSION}"
    "org.opencontainers.image.licenses"    = "Apache-2.0"
    "org.opencontainers.image.description" = "A Go API for docz repos"
  }
}

// Local development build — single-arch, loads into Docker daemon.
target "dev-api" {
  inherits = ["_common_api"]
  tags     = tags(API_IMAGE, "dev")
  output   = ["type=docker"]
}

// CI validation build — multi-arch, no push.
target "ci-api" {
  inherits   = ["_common_api"]
  tags       = tags(API_IMAGE, VERSION)
  platforms  = ["linux/amd64", "linux/arm64"]
  output     = ["type=cacheonly"]
  cache-from = ["type=gha"]
  cache-to   = ["type=gha,mode=max"]
}

// Populated by docker/metadata-action in CI with computed tags and labels.
// Default tags are used for local `make docker-push`; CI overrides via bake file merge.
target "docker-metadata-action" {
  tags = tags(API_IMAGE, VERSION)
}

// Release build — multi-arch, pushes to registry.
// Tags are inherited from docker-metadata-action (overridden by metadata-action in CI).
target "release-api" {
  inherits   = ["_common_api", "docker-metadata-action"]
  platforms  = ["linux/amd64", "linux/arm64"]
  output     = ["type=registry"]
  cache-from = ["type=gha"]
  cache-to   = ["type=gha,mode=max"]
}
