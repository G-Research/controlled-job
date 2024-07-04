{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "controlled-job.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "controlled-job.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "controlled-job.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Labels to identify the chart/owners of resources
*/}}
{{- define "controlled-job.chartLabels" -}}
helm.sh/chart: {{ include "controlled-job.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "controlled-job.labels" -}}
{{ include "controlled-job.selectorLabels" . }}
{{ include "controlled-job.chartLabels" . }}
{{- with .Values.service.extraLabels -}}
{{- toYaml . | nindent 0 -}}
{{- end -}}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "controlled-job.selectorLabels" -}}
app.kubernetes.io/name: {{ include "controlled-job.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
