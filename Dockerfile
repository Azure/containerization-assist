FROM tomcat:9-jdk11

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -v -o app-binary

FROM gcr.io/distroless/static-debian12

ENV PORT=80
EXPOSE 80

WORKDIR /app
COPY --from=builder /build/app-binary .

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:80/health || exit 1
  CMD curl -f http://localhost:80/health || exit 1
CMD ["/app/app-binary"]
