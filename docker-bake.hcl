// docker-bake.hcl — multi-arch build pipeline for the repository's images.
//
// One target family per component (DESIGN-0017 §8): `-api` builds docz-api
// from Dockerfile.api, `-ui` builds docz-site from Dockerfile.ui. Each family
// has three targets:
//   - dev-<c>:     local single-arch build, loaded into the Docker daemon
//   - ci-<c>:      multi-arch validation build, cache only, no push
//   - release-<c>: multi-arch build + push (CI only, gated on tag)
//
// Groups keep the component-free spellings working: `default` (a bare
// `docker buildx bake`) and `ci` (CI's docker-build job) build every
// component's dev/ci target.
//
// CI workflow consumes this via docker/bake-action with the `targets`
// input. The publish workflows (ghcr.yml, ecr.yml) merge in tag-derived
// image refs from docker/metadata-action's bake-file outputs, written to
// the component's docker-metadata-action-<c> target (its `bake-target`).

variable "REGISTRY" {
  default = "ghcr.io"
}

// Image name per component. Not IMAGE_NAME: ecr.yml exports an env var of
// that name, and bake reads same-named env vars into variables.
variable "API_IMAGE" {
  default = "donaldgifford/docz-api"
}

// The image keeps docz-site's name: the chart and every deployment pull it.
variable "UI_IMAGE" {
  default = "donaldgifford/docz-site"
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
  targets = ["dev-api", "dev-ui"]
}

group "ci" {
  targets = ["ci-api", "ci-ui"]
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
// Default tags are used for a local `just api docker-buildx`; CI overrides
// them via the bake-file merge.
target "docker-metadata-action-api" {
  tags = tags(API_IMAGE, VERSION)
}

// Release build — multi-arch, pushes to registry.
// Tags are inherited from docker-metadata-action-api (overridden in CI).
target "release-api" {
  inherits   = ["_common_api", "docker-metadata-action-api"]
  platforms  = ["linux/amd64", "linux/arm64"]
  output     = ["type=registry"]
  cache-from = ["type=gha"]
  cache-to   = ["type=gha,mode=max"]
}

// Base target for docz-site. The context is ui/ alone; docz-api's spec
// arrives as the named context `spec`, which Dockerfile.ui copies to
// /api/openapi.yaml so orval's ../api/openapi.yaml resolves (DESIGN-0017
// OQ 3). The dockerfile path is relative to the context.
target "_common_ui" {
  dockerfile = "../Dockerfile.ui"
  context    = "ui"
  contexts = {
    spec = "api"
  }
  labels = {
    "org.opencontainers.image.source"      = "https://github.com/donaldgifford/docz"
    "org.opencontainers.image.revision"    = "${COMMIT_SHA}"
    "org.opencontainers.image.created"     = "${BUILD_DATE}"
    "org.opencontainers.image.version"     = "${VERSION}"
    "org.opencontainers.image.licenses"    = "Apache-2.0"
    "org.opencontainers.image.description" = "A UI for docz repos"
  }
}

target "dev-ui" {
  inherits = ["_common_ui"]
  tags     = tags(UI_IMAGE, "dev")
  output   = ["type=docker"]
}

target "ci-ui" {
  inherits   = ["_common_ui"]
  tags       = tags(UI_IMAGE, VERSION)
  platforms  = ["linux/amd64", "linux/arm64"]
  output     = ["type=cacheonly"]
  cache-from = ["type=gha"]
  cache-to   = ["type=gha,mode=max"]
}

target "docker-metadata-action-ui" {
  tags = tags(UI_IMAGE, VERSION)
}

target "release-ui" {
  inherits   = ["_common_ui", "docker-metadata-action-ui"]
  platforms  = ["linux/amd64", "linux/arm64"]
  output     = ["type=registry"]
  cache-from = ["type=gha"]
  cache-to   = ["type=gha,mode=max"]
}
