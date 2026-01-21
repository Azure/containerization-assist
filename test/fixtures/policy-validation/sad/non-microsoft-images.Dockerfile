# Test: require-microsoft-images (FAIL)
# Violation: Uses Docker Hub instead of MCR
FROM docker.io/library/alpine:3.19

WORKDIR /app
COPY . .

USER app
EXPOSE 8080

CMD ["./app"]
