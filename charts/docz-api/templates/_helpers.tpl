{{/*
Expand the name of the chart.
*/}}
{{- define "docz-api.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this
(by the DNS naming spec). If release name contains chart name it will be used
as a full name.
*/}}
{{- define "docz-api.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "docz-api.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "docz-api.labels" -}}
helm.sh/chart: {{ include "docz-api.chart" . }}
{{ include "docz-api.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "docz-api.selectorLabels" -}}
app.kubernetes.io/name: {{ include "docz-api.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use.
*/}}
{{- define "docz-api.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "docz-api.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the secret to use.
*/}}
{{- define "docz-api.secretName" -}}
{{- if .Values.secrets.create }}
{{- include "docz-api.fullname" . }}
{{- else }}
{{- required "secrets.existingSecret is required when secrets.create is false" .Values.secrets.existingSecret }}
{{- end }}
{{- end }}

{{/*
Resource name for the chart-rendered Postgres deployment + Service +
PVC + Secret. Always derived from the release fullname; the chart
does not honour an existingSecret for the *baked* mode (the existing
secret is the operator's signal to use external mode).
*/}}
{{- define "docz-api.postgresFullname" -}}
{{- printf "%s-postgres" (include "docz-api.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Resource name for the chart-rendered Valkey deployment + Service +
PVC + Secret.
*/}}
{{- define "docz-api.valkeyFullname" -}}
{{- printf "%s-valkey" (include "docz-api.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Resource name for the chart-rendered Meilisearch StatefulSet + Service
+ PVC + Secret (baked mode).
*/}}
{{- define "docz-api.meiliFullname" -}}
{{- printf "%s-meilisearch" (include "docz-api.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Secret name holding DATABASE_URL. Three modes:
  - `external`: operator's store.external.existingSecret (required).
  - `baked`:    chart-rendered Secret (postgresFullname).
  - `cnpg`:     CNPG-created `<cluster>-app` Secret.
*/}}
{{- define "docz-api.storeSecretName" -}}
{{- if eq .Values.store.postgres.mode "external" -}}
{{- required "store.external.existingSecret is required when store.postgres.mode=external" .Values.store.external.existingSecret -}}
{{- else if eq .Values.store.postgres.mode "cnpg" -}}
{{- printf "%s-app" (include "docz-api.postgresFullname" .) -}}
{{- else -}}
{{- include "docz-api.postgresFullname" . -}}
{{- end -}}
{{- end -}}

{{/*
Secret key holding DATABASE_URL. CNPG always writes connection strings
under the `uri` key; baked uses the chart-controlled `DATABASE_URL`;
external honours store.external.secretKey.
*/}}
{{- define "docz-api.storeSecretKey" -}}
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
{{- define "docz-api.queueSecretName" -}}
{{- if eq .Values.queue.valkey.mode "external" -}}
{{- required "queue.external.existingSecret is required when queue.valkey.mode=external" .Values.queue.external.existingSecret -}}
{{- else -}}
{{- include "docz-api.valkeyFullname" . -}}
{{- end -}}
{{- end -}}

{{/*
Secret key holding REDIS_URL.
*/}}
{{- define "docz-api.queueSecretKey" -}}
{{- if eq .Values.queue.valkey.mode "external" -}}
{{- .Values.queue.external.secretKey | default "REDIS_URL" -}}
{{- else -}}
REDIS_URL
{{- end -}}
{{- end -}}

{{/*
MEILI_HOST value. Baked mode targets the chart-rendered headless
Service; external mode uses the operator-supplied host URL (required).
*/}}
{{- define "docz-api.meiliHost" -}}
{{- if eq .Values.search.meili.mode "external" -}}
{{- required "search.meili.host is required when search.meili.mode=external" .Values.search.meili.host -}}
{{- else -}}
{{- printf "http://%s:7700" (include "docz-api.meiliFullname" .) -}}
{{- end -}}
{{- end -}}

{{/*
Secret name holding the Meilisearch key. Three cases:
  - `external`:            operator's search.meili.external.existingSecret.
  - `baked` + existingSecret: operator's search.meili.existingSecret
    (baked Meilisearch and docz-api share it; no chart Secret rendered).
  - `baked` (default):     chart-rendered Secret (meiliFullname).
*/}}
{{- define "docz-api.searchSecretName" -}}
{{- if eq .Values.search.meili.mode "external" -}}
{{- required "search.meili.external.existingSecret is required when search.meili.mode=external" .Values.search.meili.external.existingSecret -}}
{{- else if .Values.search.meili.existingSecret -}}
{{- .Values.search.meili.existingSecret -}}
{{- else -}}
{{- include "docz-api.meiliFullname" . -}}
{{- end -}}
{{- end -}}

{{/*
Secret key holding the Meilisearch key. external honours
external.secretKey; baked+existingSecret honours existingSecretKey; the
chart-rendered baked Secret uses MEILI_API_KEY.
*/}}
{{- define "docz-api.searchSecretKey" -}}
{{- if eq .Values.search.meili.mode "external" -}}
{{- .Values.search.meili.external.secretKey | default "MEILI_API_KEY" -}}
{{- else if .Values.search.meili.existingSecret -}}
{{- .Values.search.meili.existingSecretKey | default "MEILI_API_KEY" -}}
{{- else -}}
MEILI_API_KEY
{{- end -}}
{{- end -}}
