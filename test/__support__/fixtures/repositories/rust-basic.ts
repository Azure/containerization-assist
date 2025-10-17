/**
 * Rust Basic Repository Fixture
 * Simple Rust web application for testing
 */

export const rustBasicRepository = {
  'Cargo.toml': `[package]
name = "rust-basic"
version = "1.0.0"
edition = "2021"

[dependencies]
tokio = { version = "1", features = ["full"] }
warp = "0.3"
serde = { version = "1.0", features = ["derive"] }
serde_json = "1.0"`,
  'src/main.rs': `use std::env;
use std::convert::Infallible;
use serde::{Deserialize, Serialize};
use warp::Filter;

#[derive(Serialize, Deserialize)]
struct HelloResponse {
    message: String,
    timestamp: String,
}

#[derive(Serialize, Deserialize)]
struct HealthResponse {
    status: String,
    version: String,
}

async fn hello_handler() -> Result<impl warp::Reply, Infallible> {
    let response = HelloResponse {
        message: "Hello World!".to_string(),
        timestamp: chrono::Utc::now().to_rfc3339(),
    };
    Ok(warp::reply::json(&response))
}

async fn health_handler() -> Result<impl warp::Reply, Infallible> {
    let response = HealthResponse {
        status: "healthy".to_string(),
        version: "1.0.0".to_string(),
    };
    Ok(warp::reply::json(&response))
}

#[tokio::main]
async fn main() {
    let hello = warp::path::end()
        .and(warp::get())
        .and_then(hello_handler);

    let health = warp::path("health")
        .and(warp::get())
        .and_then(health_handler);

    let routes = hello.or(health);

    let port: u16 = env::var("PORT")
        .unwrap_or_else(|_| "3030".to_string())
        .parse()
        .expect("PORT must be a valid number");

    println!("Server starting on port {}", port);
    warp::serve(routes)
        .run(([0, 0, 0, 0], port))
        .await;
}`,
  'README.md': `# Rust Basic

A simple Rust web application using Warp for testing containerization.

## Running the application

\`\`\`bash
cargo run
\`\`\`

The server will start on port 3030.`,
  '.gitignore': `/target
Cargo.lock
**/*.rs.bk
*.pdb`,
};

export const expectedRustBasicAnalysis = {
  projectType: 'rust',
  packageManager: 'cargo',
  buildTool: 'cargo',
  crateName: 'rust-basic',
  dependencies: ['tokio', 'warp', 'serde', 'serde_json'],
  devDependencies: [],
  rustEdition: '2021',
  ports: [3030],
  hasDockerfile: false,
  hasTests: false,
  entrypoint: 'src/main.rs',
};

export const expectedRustBasicDockerfile = `FROM rust:1.75 AS builder

WORKDIR /app
COPY Cargo.toml Cargo.lock ./
RUN mkdir src && echo "fn main() {}" > src/main.rs && cargo build --release && rm -rf src

COPY src src
RUN touch src/main.rs && cargo build --release

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/target/release/rust-basic .

EXPOSE 3030

RUN useradd -r -s /bin/false appuser
USER appuser

CMD ["./rust-basic"]`;

export {};