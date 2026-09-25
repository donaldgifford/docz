{{/*
Shared helpers for every template in the chart (DESIGN-0018 §2, §6).

Helpers that describe one workload take a dict, never the root context:

  include "docz.labels" (dict "ctx" $ "component" "api")

`component` is the workload's role (api, site, postgres, valkey,
meilisearch, test). It becomes `app.kubernetes.io/component`, which is in
every selector, and for api/site it is also the values block the helper
reads (.Values.api, .Values.site).
*/}}

{{/*
Expand the name of the chart.
*/}}
{{- define "docz.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this
(by the DNS naming spec). If release name contains chart name it will be used
as a full name.
*/}}
{{- define "docz.fullname" -}}
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
{{- define "docz.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
<fullname>-<component>: the name of every object a workload owns.
*/}}
{{- define "docz.componentFullname" -}}
{{- printf "%s-%s" (include "docz.fullname" .ctx) .component | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Selector labels. The component is required, so a selector that could match
another workload's pods cannot render (DESIGN-0018 §2).
*/}}
{{- define "docz.selectorLabels" -}}
app.kubernetes.io/name: {{ include "docz.name" .ctx }}
app.kubernetes.io/instance: {{ .ctx.Release.Name }}
app.kubernetes.io/component: {{ required "docz.selectorLabels: component is required" .component }}
{{- end }}

{{/*
extraLabels: a tracking hook, appended to every object's metadata and to pod
templates, never to a selector, a volumeClaimTemplate, or CNPG
inheritedMetadata (DESIGN-0018 §2). A key the chart sets itself fails the
render rather than silently replacing a label a selector depends on.
Takes the root context.
*/}}
{{- define "docz.extraLabels" -}}
{{- $reserved := list "helm.sh/chart" "app.kubernetes.io/name" "app.kubernetes.io/instance" "app.kubernetes.io/component" "app.kubernetes.io/version" "app.kubernetes.io/managed-by" }}
{{- with .Values.extraLabels }}
{{- range $k, $_ := . }}
{{- if has $k $reserved }}
{{- fail (printf "extraLabels: %q is set by the chart and cannot be overridden" $k) }}
{{- end }}
{{- end }}
{{- toYaml . }}
{{- end }}
{{- end }}

{{/*
Common labels for an object's own metadata.
*/}}
{{- define "docz.labels" -}}
helm.sh/chart: {{ include "docz.chart" .ctx }}
{{ include "docz.selectorLabels" . }}
{{- if .ctx.Chart.AppVersion }}
app.kubernetes.io/version: {{ .ctx.Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .ctx.Release.Service }}
{{- with (include "docz.extraLabels" .ctx) }}
{{ . }}
{{- end }}
{{- end }}

{{/*
Pod-template labels: the selector labels plus extraLabels. A workload adds
its own podLabels next to this.
*/}}
{{- define "docz.podLabels" -}}
{{ include "docz.selectorLabels" . }}
{{- with (include "docz.extraLabels" .ctx) }}
{{ . }}
{{- end }}
{{- end }}

{{/*
The ServiceAccount a workload (api or site) runs as.
*/}}
{{- define "docz.serviceAccountName" -}}
{{- $sa := (index .ctx.Values .component).serviceAccount }}
{{- if $sa.create }}
{{- default (include "docz.componentFullname" .) $sa.name }}
{{- else }}
{{- default "default" $sa.name }}
{{- end }}
{{- end }}

{{/*
A workload's image reference. Both images default their tag to the one
appVersion, so a single bump moves both (DESIGN-0018 Goals).
*/}}
{{- define "docz.image" -}}
{{- $image := (index .ctx.Values .component).image }}
{{- printf "%s:%s" $image.repository (default .ctx.Chart.AppVersion $image.tag) }}
{{- end }}
