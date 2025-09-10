/**
 * Go Basic Repository Fixture
 * Simple Go web application for testing
 */

export const goBasicRepository = {
  'go.mod': `module go-basic

go 1.21

require (
    github.com/gorilla/mux v1.8.0
)`,
  'main.go': `package main

import (
    "encoding/json"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/gorilla/mux"
)

type Response struct {
    Message   string    \`json:"message"\`
    Timestamp time.Time \`json:"timestamp"\`
}

type HealthResponse struct {
    Status  string \`json:"status"\`
    Version string \`json:"version"\`
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    response := Response{
        Message:   "Hello World!",
        Timestamp: time.Now(),
    }
    json.NewEncoder(w).Encode(response)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    response := HealthResponse{
        Status:  "healthy",
        Version: "1.0.0",
    }
    json.NewEncoder(w).Encode(response)
}

func main() {
    r := mux.NewRouter()
    r.HandleFunc("/", helloHandler).Methods("GET")
    r.HandleFunc("/health", healthHandler).Methods("GET")

    port := os.Getenv("PORT")
    if port == "" {
        port = "8000"
    }

    log.Printf("Server starting on port %s", port)
    log.Fatal(http.ListenAndServe(":"+port, r))
}`,
  'README.md': `# Go Basic

A simple Go web application for testing containerization.

## Running the application

\`\`\`bash
go mod download
go run main.go
\`\`\`

The server will start on port 8000.`,
  '.gitignore': `# Binaries for programs and plugins
*.exe
*.exe~
*.dll
*.so
*.dylib

# Test binary, built with \`go test -c\`
*.test

# Output of the go coverage tool
*.out

# Dependency directories
vendor/

# Go workspace file
go.work`,
};

export const expectedGoBasicAnalysis = {
  projectType: 'go',
  packageManager: 'go',
  buildTool: 'go',
  moduleName: 'go-basic',
  dependencies: ['github.com/gorilla/mux'],
  devDependencies: [],
  goVersion: '1.21',
  ports: [8000],
  hasDockerfile: false,
  hasTests: false,
  entrypoint: 'main.go',
};

export const expectedGoBasicDockerfile = `FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/main .

EXPOSE 8000

RUN adduser -D -s /bin/sh appuser
USER appuser

CMD ["./main"]`;

export {};