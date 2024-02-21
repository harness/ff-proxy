{{/*
Expand the name of the chart.
*/}}
{{- define "ff-proxy.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "ff-proxy.fullname" -}}
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
{{- define "ff-proxy.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "ff-proxy.labels" -}}
helm.sh/chart: {{ include "ff-proxy.chart" . }}
app.kubernetes.io/name: {{ include "ff-proxy.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Writer selector labels
*/}}
{{- define "ff-proxy.writer.SelectorLabels" -}}
app.kubernetes.io/component: {{ include "ff-proxy.name" . }}-writer
{{- end }}

{{/*
Read replica selector labels
*/}}
{{- define "ff-proxy.readReplica.SelectorLabels" -}}
app.kubernetes.io/component: {{ include "ff-proxy.name" . }}-read-replica
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "ff-proxy.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "ff-proxy.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Define the proxy key name
*/}}
{{- define "ff-proxy.proxyKey" -}}
{{- if not .Values.existingProxyKey }}
{{- include "ff-proxy.fullname" . }}-proxy-key
{{- else }}
{{- .Values.existingProxyKey | trunc 63 | toString }}
{{- end }}
{{- end }}

{{/*
Define the auth secret name
*/}}
{{- define "ff-proxy.authSecret" -}}
{{- if not .Values.existingAuthSecret }}
{{- include "ff-proxy.fullname" . }}-auth-secret
{{- else }}
{{- .Values.existingAuthSecret | trunc 63 | toString }}
{{- end }}
{{- end }}

{{/*
Define the redis password name
*/}}
{{- define "ff-proxy.redisPassword" -}}
{{- if not .Values.redis.existingPassword }}
{{- include "ff-proxy.fullname" . }}-redis-password
{{- else }}
{{- .Values.redis.existingPassword | trunc 63 | toString }}
{{- end }}
{{- end }}

{{/*
Define resource names
*/}}
{{- define "ff-proxy.writer.name" -}}
{{- include "ff-proxy.fullname" . }}-writer
{{- end }}
{{- define "ff-proxy.readReplica.name" -}}
{{- include "ff-proxy.fullname" . }}-read-replica
{{- end }}
