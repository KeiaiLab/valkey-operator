{{/*
Expand the name of the chart.
*/}}
{{- define "valkey-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "valkey-operator.fullname" -}}
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
{{- define "valkey-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "valkey-operator.labels" -}}
helm.sh/chart: {{ include "valkey-operator.chart" . }}
{{ include "valkey-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: valkey-operator
{{- end }}

{{/*
Selector labels
*/}}
{{- define "valkey-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "valkey-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: controller-manager
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "valkey-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "valkey-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the leader election lease
*/}}
{{- define "valkey-operator.leaderElectionId" -}}
{{- printf "%s-leader-election" (include "valkey-operator.fullname" .) }}
{{- end }}

{{/*
Create the leader election namespace
*/}}
{{- define "valkey-operator.leaderElectionNamespace" -}}
{{- if .Values.leaderElection.namespace }}
{{- .Values.leaderElection.namespace }}
{{- else }}
{{- .Release.Namespace }}
{{- end }}
{{- end }}

{{/*
Operator image
*/}}
{{- define "valkey-operator.image" -}}
{{- $tag := default .Chart.AppVersion .Values.image.tag }}
{{- printf "%s:%s" .Values.image.repository $tag }}
{{- end }}
