# Test: require-workdir (PASS)
# Includes WORKDIR directive
FROM mcr.microsoft.com/dotnet/aspnet:8.0

WORKDIR /app
COPY . .

RUN apt-get update && apt-get install -y --no-install-recommends curl && rm -rf /var/lib/apt/lists/*

USER app
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s CMD curl --fail http://localhost:8080/health || exit 1

CMD ["dotnet", "MyApp.dll"]
