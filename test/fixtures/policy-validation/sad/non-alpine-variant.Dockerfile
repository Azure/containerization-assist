# Test: recommend-alpine (WARN)
# Uses non-Alpine Python variant from MCR
FROM mcr.microsoft.com/mirror/docker/library/python:3.11

WORKDIR /app
COPY . .

RUN apt-get update && apt-get install -y --no-install-recommends curl && rm -rf /var/lib/apt/lists/*

USER app
EXPOSE 8000

HEALTHCHECK --interval=30s --timeout=3s CMD curl --fail http://localhost:8000/health || exit 1

CMD ["python", "app.py"]
