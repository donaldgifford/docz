# {{ .SiteName }}

Welcome to the documentation for {{ .SiteName }}.

## Document Types

{{ range .Types }}- [{{ .NavTitle }}]({{ .Dir }}/README.md)
{{ end }}
