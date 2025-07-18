package dockerfile

// RustTemplate provides the Dockerfile template for Rust applications
const RustTemplate = `# Build stage
FROM rust:{{.LanguageVersion}} AS builder
WORKDIR /app
COPY Cargo.toml Cargo.lock ./
RUN mkdir src && echo 'fn main() {}' > src/main.rs
RUN cargo build --release
COPY . .
RUN cargo build --release

# Runtime stage
FROM debian:bookworm-slim
WORKDIR /app
COPY --from=builder /app/target/release/* ./
{{if .Port -}}
EXPOSE {{.Port}}
{{end -}}
CMD ["./main"]
`
