package dockerfile

// GenericTemplate provides the Dockerfile template for generic/unknown applications
const GenericTemplate = `FROM alpine:latest
WORKDIR /app
COPY . .
{{if .Port -}}
EXPOSE {{.Port}}
{{end -}}
CMD ["./start.sh"]
`
