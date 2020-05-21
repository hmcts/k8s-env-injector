{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "chart-env-injector.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "chart-env-injector.fullname" -}}
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
{{- define "chart-env-injector.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "chart-env-injector.labels" }}
app.kubernetes.io/name: {{ include "chart-env-injector.name" . }}
helm.sh/chart: {{ include "chart-env-injector.chart" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Template to add the environment variable list and checking the format of the keys
The key or "environment variable" must be uppercase and contain only numbers or "_".
*/}}
{{- define "chart-env-injector.environment" -}}
  {{- if .Values.environment -}}
    {{- range $key, $val := .Values.environment }}
- name: {{ if $key | regexMatch "^[^.-]+$" -}}
          {{- $key }}
        {{- else -}}
            {{- fail (join "Environment variables can not contain '.' or '-' Failed key: " ($key|quote)) -}}
        {{- end }}
  value: {{ tpl ($val | quote) $ }}
    {{- end }}
  {{- end }}
{{- end }}

{{/*
Template to add the dns options
*/}}
{{- define "chart-env-injector.dnsOptions" -}}
  {{- if .Values.dnsOptions -}}
    {{- range $key, $val := .Values.dnsOptions }}
- name: {{ $key }}
      {{- if $val }}
  value: {{ tpl ($val | quote) $ }}
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}
