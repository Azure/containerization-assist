# Test: recommend-alpine (PASS)
# Uses Alpine variant
FROM mcr.microsoft.com/dotnet/runtime:8.0-alpine

WORKDIR /app
COPY . .

RUN apk add --no-cache curl

USER app
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s CMD curl --fail http://localhost:8080/health || exit 1

CMD ["dotnet", "MyApp.dll"]
