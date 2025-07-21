package dockerfile

// GoTemplate provides the Dockerfile template for Go applications
const GoTemplate = `# Build stage
FROM golang:{{.LanguageVersion}}-alpine AS builder
WORKDIR /app
# Copy go.mod first, go.sum if it exists
COPY go.mod ./
# Copy go.sum only if it exists (using wildcard that doesn't fail if missing)
# The pattern 'go.su[m]' matches 'go.sum' and ensures the command succeeds even if the file is absent.
COPY go.su[m] ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Runtime stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
{{if .Port -}}
EXPOSE {{.Port}}
{{end -}}
CMD ["./main"]
`
