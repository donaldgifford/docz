// docker-bake.hcl — multi-arch build pipeline for docz-api.
//
// Targets:
//   - default: local single-arch build (used by `docker buildx bake`)
//   - ci:      linux/amd64 build + push of `:dev-ci` for PR validation
//   - release: multi-arch build + push to GHCR (CI only, gated on tag)
//
// CI workflow consumes this via docker/bake-action@v6 with the `targets`
// input. The release workflow merges in tag-derived image refs from
// docker/metadata-action's bake-file outputs.

variable "REGISTRY" {
  default = "ghcr.io"
}

variable "IMAGE_NAME" {
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
  params = [version]
  result = version == "dev" ? [
    "${REGISTRY}/${IMAGE_NAME}:dev",
    ] : [
    "${REGISTRY}/${IMAGE_NAME}:${version}",
    "${REGISTRY}/${IMAGE_NAME}:latest",
  ]
}

// Base target with shared configuration.
target "_common" {
  dockerfile = "Dockerfile"
  context    = "."
  // Build args feed the Dockerfile's VERSION/COMMIT/DATE ARGs, which the
  // build injects via -ldflags into main.version/commit/date. The bake
  // variables (VERSION/COMMIT_SHA/BUILD_DATE) are set by the publish
  // workflows; without this block every image compiled in version=dev.
  args = {
    VERSION = "${VERSION}"
    COMMIT  = "${COMMIT_SHA}"
    DATE    = "${BUILD_DATE}"
  }
  labels = {
    "org.opencontainers.image.source"      = "https://github.com/donaldgifford/docz-api"
    "org.opencontainers.image.revision"    = "${COMMIT_SHA}"
    "org.opencontainers.image.created"     = "${BUILD_DATE}"
    "org.opencontainers.image.version"     = "${VERSION}"
    "org.opencontainers.image.licenses"    = "Apache-2.0"
    "org.opencontainers.image.description" = "A Go API for docz repos"
  }
}

// Local development build — single-arch, loads into Docker daemon.
target "dev" {
  inherits = ["_common"]
  tags     = tags("dev")
  output   = ["type=docker"]
}

// CI validation build — multi-arch, no push.
target "ci" {
  inherits   = ["_common"]
  tags       = tags(VERSION)
  platforms  = ["linux/amd64", "linux/arm64"]
  output     = ["type=cacheonly"]
  cache-from = ["type=gha"]
  cache-to   = ["type=gha,mode=max"]
}

// Populated by docker/metadata-action in CI with computed tags and labels.
// Default tags are used for local `make docker-push`; CI overrides via bake file merge.
target "docker-metadata-action" {
  tags = tags(VERSION)
}

// Release build — multi-arch, pushes to registry.
// Tags are inherited from docker-metadata-action (overridden by metadata-action in CI).
target "release" {
  inherits   = ["_common", "docker-metadata-action"]
  platforms  = ["linux/amd64", "linux/arm64"]
  output     = ["type=registry"]
  cache-from = ["type=gha"]
  cache-to   = ["type=gha,mode=max"]
}
