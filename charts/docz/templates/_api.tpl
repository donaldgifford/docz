{{/*
The API's dependency helpers (DESIGN-0018 §6): its Secret, the three
backends' names and connection Secrets, and the login-provider list. Ported
from charts/docz-api's `docz-api.*` helpers with three changes: renamed
`docz.api.*`, the provider list read from the shared `auth.providers`, and
the API Secret named `<fullname>-api`. All take the root context.
*/}}

{{/*
Create the name of the secret to use.
*/}}
{{- define "docz.api.secretName" -}}
{{- if .Values.api.secrets.create }}
{{- include "docz.componentFullname" (dict "ctx" . "component" "api") }}
{{- else }}
{{- required "api.secrets.existingSecret is required when api.secrets.create is false" .Values.api.secrets.existingSecret }}
{{- end }}
{{- end }}

{{/*
Resource name for the chart-rendered Postgres deployment + Service +
PVC + Secret. Always derived from the release fullname; the chart
does not honour an existingSecret for the *baked* mode (the existing
secret is the operator's signal to use external mode).
*/}}
{{- define "docz.api.postgresFullname" -}}
{{- printf "%s-postgres" (include "docz.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Resource name for the chart-rendered Valkey deployment + Service +
PVC + Secret.
*/}}
{{- define "docz.api.valkeyFullname" -}}
{{- printf "%s-valkey" (include "docz.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Resource name for the chart-rendered Meilisearch StatefulSet + Service
+ PVC + Secret (baked mode).
*/}}
{{- define "docz.api.meiliFullname" -}}
{{- printf "%s-meilisearch" (include "docz.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Secret name holding DATABASE_URL. Three modes:
  - `external`: operator's store.external.existingSecret (required).
  - `baked`:    chart-rendered Secret (postgresFullname).
  - `cnpg`:     CNPG-created `<cluster>-app` Secret.
*/}}
{{- define "docz.api.storeSecretName" -}}
{{- if eq .Values.store.postgres.mode "external" -}}
{{- required "store.external.existingSecret is required when store.postgres.mode=external" .Values.store.external.existingSecret -}}
{{- else if eq .Values.store.postgres.mode "cnpg" -}}
{{- printf "%s-app" (include "docz.api.postgresFullname" .) -}}
{{- else -}}
{{- include "docz.api.postgresFullname" . -}}
{{- end -}}
{{- end -}}

{{/*
Secret key holding DATABASE_URL. CNPG always writes connection strings
under the `uri` key; baked uses the chart-controlled `DATABASE_URL`;
external honours store.external.secretKey.
*/}}
{{- define "docz.api.storeSecretKey" -}}
{{- if eq .Values.store.postgres.mode "external" -}}
{{- .Values.store.external.secretKey | default "DATABASE_URL" -}}
{{- else if eq .Values.store.postgres.mode "cnpg" -}}
uri
{{- else -}}
DATABASE_URL
{{- end -}}
{{- end -}}

{{/*
Secret name holding REDIS_URL.
*/}}
{{- define "docz.api.queueSecretName" -}}
{{- if eq .Values.queue.valkey.mode "external" -}}
{{- required "queue.external.existingSecret is required when queue.valkey.mode=external" .Values.queue.external.existingSecret -}}
{{- else -}}
{{- include "docz.api.valkeyFullname" . -}}
{{- end -}}
{{- end -}}

{{/*
Secret key holding REDIS_URL.
*/}}
{{- define "docz.api.queueSecretKey" -}}
{{- if eq .Values.queue.valkey.mode "external" -}}
{{- .Values.queue.external.secretKey | default "REDIS_URL" -}}
{{- else -}}
REDIS_URL
{{- end -}}
{{- end -}}

{{/*
Secret name holding the baked Valkey password (the server's `requirepass`
and the basis for docz-api's REDIS_URL). Honours
queue.valkey.existingSecret; otherwise the chart-rendered Secret.
*/}}
{{- define "docz.api.valkeyPasswordSecretName" -}}
{{- if .Values.queue.valkey.existingSecret -}}
{{- .Values.queue.valkey.existingSecret -}}
{{- else -}}
{{- include "docz.api.valkeyFullname" . -}}
{{- end -}}
{{- end -}}

{{/*
Secret key holding the baked Valkey password. Honours existingSecretKey
when an existingSecret is set; the chart-rendered Secret uses VALKEY_PASSWORD.
*/}}
{{- define "docz.api.valkeyPasswordSecretKey" -}}
{{- .Values.queue.valkey.existingSecretKey | default "VALKEY_PASSWORD" -}}
{{- end -}}

{{/*
REDIS_URL for docz-api in baked mode. The password is injected at runtime
via the container's VALKEY_PASSWORD env (Kubernetes $(VAR) interpolation),
so no plaintext DSN is stored — the chart never needs to read the password
value, which `helm template` couldn't do for an existingSecret anyway.
*/}}
{{- define "docz.api.valkeyBakedDsn" -}}
{{- printf "redis://:$(VALKEY_PASSWORD)@%s.%s.svc.cluster.local:6379/0" (include "docz.api.valkeyFullname" .) .Release.Namespace -}}
{{- end -}}

{{/*
MEILI_HOST value. Baked mode targets the chart-rendered headless
Service; external mode uses the operator-supplied host URL (required).
*/}}
{{- define "docz.api.meiliHost" -}}
{{- if eq .Values.search.meili.mode "external" -}}
{{- required "search.meili.host is required when search.meili.mode=external" .Values.search.meili.host -}}
{{- else -}}
{{- printf "http://%s:7700" (include "docz.api.meiliFullname" .) -}}
{{- end -}}
{{- end -}}

{{/*
Secret name holding the Meilisearch key. Three cases:
  - `external`:            operator's search.meili.external.existingSecret.
  - `baked` + existingSecret: operator's search.meili.existingSecret
    (baked Meilisearch and docz-api share it; no chart Secret rendered).
  - `baked` (default):     chart-rendered Secret (meiliFullname).
*/}}
{{- define "docz.api.searchSecretName" -}}
{{- if eq .Values.search.meili.mode "external" -}}
{{- required "search.meili.external.existingSecret is required when search.meili.mode=external" .Values.search.meili.external.existingSecret -}}
{{- else if .Values.search.meili.existingSecret -}}
{{- .Values.search.meili.existingSecret -}}
{{- else -}}
{{- include "docz.api.meiliFullname" . -}}
{{- end -}}
{{- end -}}

{{/*
Secret key holding the Meilisearch key. external honours
external.secretKey; baked+existingSecret honours existingSecretKey; the
chart-rendered baked Secret uses MEILI_API_KEY.
*/}}
{{- define "docz.api.searchSecretKey" -}}
{{- if eq .Values.search.meili.mode "external" -}}
{{- .Values.search.meili.external.secretKey | default "MEILI_API_KEY" -}}
{{- else if .Values.search.meili.existingSecret -}}
{{- .Values.search.meili.existingSecretKey | default "MEILI_API_KEY" -}}
{{- else -}}
MEILI_API_KEY
{{- end -}}
{{- end -}}

{{/*
Normalized login-provider list from auth.providers (comma-separated,
whitespace stripped, empties dropped) as a JSON array. Consume with
`fromJsonArray`:

  {{- $providers := include "docz.api.authProviders" . | fromJsonArray }}
  {{- if has "okta" $providers }}

Each enabled provider gates its own env block in the Deployment and its own
client-secret key in the Secret, so a github-only install carries no OKTA_*
env and no okta-client-secret key.
*/}}
{{- define "docz.api.authProviders" -}}
{{- splitList "," (.Values.auth.providers | default "" | replace " " "") | compact | toJson -}}
{{- end -}}

{{/*
Whether site login is disabled entirely (auth.providers: "none").
Renders "true" when so, and empty otherwise, so it reads as a boolean:

  {{- if not (include "docz.api.authDisabled" .) }}

"none" must be the only entry, mirroring config.AuthDisabled() in the binary —
otherwise the chart would drop the session secret and redirect base while still
rendering a provider's env, producing a manifest that installs cleanly and then
crash-loops. The render fails instead.

Under none-mode the API serves every request as a synthetic anonymous identity
and mounts no login routes, so the redirect base, session secret, and every
provider credential stop being required. It is the first-setup shape — get
GitHub App ingestion working before configuring a login provider — and it
leaves the read API open to anyone who can reach the Service.
*/}}
{{- define "docz.api.authDisabled" -}}
{{- $providers := include "docz.api.authProviders" . | fromJsonArray -}}
{{- if has "none" $providers -}}
{{- if gt (len $providers) 1 -}}
{{- fail "auth.providers: \"none\" must be the only entry when present" -}}
{{- end -}}
true
{{- end -}}
{{- end -}}

{{/*
The API's in-cluster URL, the site's DOCZ_API_URL unless
site.config.doczApiUrl overrides it (DESIGN-0018 §4).
*/}}
{{- define "docz.api.internalUrl" -}}
{{- printf "http://%s.%s.svc.cluster.local:%v" (include "docz.componentFullname" (dict "ctx" . "component" "api")) .Release.Namespace .Values.api.service.port -}}
{{- end -}}
