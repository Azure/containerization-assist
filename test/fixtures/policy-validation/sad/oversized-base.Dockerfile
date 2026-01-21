# Test: block-oversized-base (WARN)
# Uses oversized base image (full Ubuntu from MCR)
FROM mcr.microsoft.com/mirror/docker/library/ubuntu:22.04

RUN apt-get update && apt-get install -y python3 python3-pip curl && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY . .

USER app
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s CMD curl --fail http://localhost:8080/health || exit 1

CMD ["python3", "app.py"]
