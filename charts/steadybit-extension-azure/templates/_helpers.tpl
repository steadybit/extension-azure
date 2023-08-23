{{/* vim: set filetype=mustache: */}}

{{/*
Expand the name of the chart.
*/}}
{{- define "azure.secret.name" -}}
{{- default "steadybit-extension-azure" .Values.azure.existingSecret -}}
{{- end -}}
