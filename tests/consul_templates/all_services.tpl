{{ range services -}}
{{ .Name }}:
{{- range service .Name }}
  {{ .Address }}
{{- end }}

{{ end -}}
