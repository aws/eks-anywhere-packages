{{/*
Create image name
*/}}
{{- define "template.image" -}}
{{- printf "/%s@%s" .repository .digest -}}
{{- end -}}
