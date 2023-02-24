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
