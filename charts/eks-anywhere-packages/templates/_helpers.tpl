{{/*
Expand the name of the chart.
*/}}
{{- define "eks-anywhere-packages.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "eks-anywhere-packages.fullname" -}}
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
{{- define "eks-anywhere-packages.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "eks-anywhere-packages.labels" -}}
helm.sh/chart: {{ include "eks-anywhere-packages.chart" . }}
{{ include "eks-anywhere-packages.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.additionalLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "eks-anywhere-packages.selectorLabels" -}}
app.kubernetes.io/name: {{ include "eks-anywhere-packages.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "eks-anywhere-packages.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "eks-anywhere-packages.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}


{{/*
Define the eks-anywhere-packages.namespace template if set with namespace or .Release.Namespace is set
*/}}
{{- define "eks-anywhere-packages.namespace" -}}
{{- if .Values.namespace -}}
{{ printf "namespace: %s" .Values.namespace }}
{{- else -}}
{{ printf "namespace: %s" .Release.Namespace }}
{{- end -}}
{{- end -}}


{{/*
Create image name
*/}}
{{- define "template.image" -}}
{{- if eq (substr 0 7 .tag) "sha256:" -}}
{{- printf "/%s@%s" .repository .tag -}}
{{- else -}}
{{- printf "/%s:%s" .repository .tag -}}
{{- end -}}
{{- end -}}

{{/*
Function to figure out os name
*/}}
{{- define "template.getOSName" -}}
{{- with first ((lookup "v1" "Node" "" "").items) -}}
{{- if contains "Bottlerocket" .status.nodeInfo.osImage -}}
{{- printf "bottlerocket" -}}
{{- else if contains "Amazon Linux" .status.nodeInfo.osImage -}}
{{- printf "docker" -}}
{{- else -}}
{{- printf "other" -}}
{{- end }}
{{- end }}
{{- end }}

{{/*
Function to figure out Bottlerocket version
*/}}
{{- define "template.getBRVersion" -}}
{{- with first ((lookup "v1" "Node" "" "").items) -}}
{{- if contains "Bottlerocket" .status.nodeInfo.osImage -}}
{{- $parts := split " " .status.nodeInfo.osImage -}}
{{- printf $parts._2 -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Function to figure out if to install cronjob, credential-package, or none
*/}}
{{- define "lookup-credential.method" -}}
    {{- if or ((lookup "v1" "Secret" "eksa-packages" "aws-secret")) (not (eq .Values.awsSecret.secret ""))  -}}
        {{- if not .Values.cronjob.suspend -}}
            {{- printf "cronjob" -}}
        {{- else -}}
            {{- $os := include "template.getOSName" . -}}
            {{- if eq $os "bottlerocket" -}}
                {{- $v := include "template.getBRVersion" . -}}
                {{- if semverCompare ">=1.25-0" .Capabilities.KubeVersion.GitVersion -}}
                    {{- if semverCompare "<=1.13" $v -}}
                        {{- printf "cronjob" -}}
                    {{- else -}}
                        {{- printf "credential-package" -}}
                    {{- end -}}
                {{- else -}}
                    {{- if semverCompare "<=1.11" $v -}}
                        {{- printf "cronjob" -}}
                    {{- else -}}
                        {{- printf "credential-package" -}}
                    {{- end -}}
                {{- end -}}
            {{- else -}}
                {{- printf "credential-package" -}}
            {{- end -}}
        {{- end -}}
    {{- else -}}
        {{- printf "none" -}}
    {{- end -}}
{{- end -}}
