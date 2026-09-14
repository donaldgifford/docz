# Helm Chart Changelog

Changes to the `docz-api` Helm chart only. For application-level changes,
see the root [CHANGELOG.md](../../CHANGELOG.md).

## 0.7.1

### Fixed

- **Chart `0.7.0` published before the image its `appVersion` names existed.**
  It declares `appVersion: "0.10.0"`, but the release that carried it was
  tagged `v0.9.1`, so `ghcr.io/donaldgifford/docz-api:0.10.0` was absent and
  a default install would `ImagePullBackOff`. App `v0.10.0` publishes that
  image, which repairs `0.7.0` in place — anyone already pinned to it needs
  no action. This release exists so the fix ships as its own chart version
  rather than rewriting bytes already published under `0.7.0`.

The cause was release-process only; no template, value, or default changed
between `0.7.0` and `0.7.1`.

## 0.7.0

### Changed

- **`appVersion` `0.6.0` → `0.10.0`**, which moves the default image tag.
  It had not been bumped since chart `0.5.1` (PR #20), so a `helm install`
  that did not override `image.tag` deployed an image predating app
  `v0.7.0` — missing the whole `api:`-block pages surface, the
  `config_snapshot` key-spelling fix, the type-dir README and frontmatter
  reporting fixes, and the OIDC scope fix that unblocked Okta and Keycloak
  clients without a `groups` scope. **Anyone on the chart default has been
  four app minors behind; upgrading to `0.7.0` moves them across all of
  it.** Pin `image.tag` if that is not wanted yet.
- App `v0.10.0` itself adds `created` and `updated_at` to search hits plus
  `sort` and `source` query parameters (OpenAPI `1.5.0`). No chart-visible
  config: no new values, no new env, no migration.

### Notes

- `deployment_test.yaml` pins the rendered default tag, so the assertion
  moved with `appVersion`. It also asserts the tag is **bare semver** — a
  `v`-prefixed `appVersion` renders an image ref that 404s, which is what
  PR #20 fixed.
