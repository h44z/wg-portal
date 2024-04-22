{{/*
Expand the name of the chart
*/}}
{{- define "wg-portal.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "wg-portal.fullname" -}}
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
Create chart name and version as used by the chart label
*/}}
{{- define "wg-portal.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "wg-portal.labels" -}}
helm.sh/chart: {{ include "wg-portal.chart" . }}
{{ include "wg-portal.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "wg-portal.selectorLabels" -}}
app.kubernetes.io/name: {{ include "wg-portal.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "wg-portal.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "wg-portal.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Define default admin credentials
If external auth is enabled and has admin group mappings,
the admin_user and admin_password values are not used.
*/}}
{{- define "wg-portal.admin" -}}
{{- $externalAdmin := false -}}
{{- with .Values.config.auth -}}
  {{- range (default list .ldap) -}}
    {{- if hasKey . "admin_group" -}}
      {{- $externalAdmin = true -}}
    {{- end -}}
  {{- end }}
  {{- range (concat (default list .oidc) (default list .oauth)) -}}
    {{- if hasKey .field_map "is_admin" -}}
      {{- $externalAdmin = true -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- if not $externalAdmin -}}
admin_user: admin@wgportal.local
admin_password: {{ printf "%s/%s" .Release.Name .Release.Namespace | b64enc }}
{{- end -}}
{{- end -}}

{{/*
Define PersistentVolumeClaim spec
*/}}
{{- define "wg-portal.pvc" -}}
accessModes: [{{ .Values.persistence.accessMode }}]
{{- with .Values.persistence.storageClass }}
storageClassName: {{ . }}
{{- end }}
resources:
  requests:
    storage: {{ .Values.persistence.size | quote }}
{{- end -}}
