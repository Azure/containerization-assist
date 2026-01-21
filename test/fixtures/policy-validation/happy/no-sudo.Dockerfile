# Test: Elevated privileges check (PASS)
# Does not use privileged commands
FROM mcr.microsoft.com/dotnet/aspnet:8.0

WORKDIR /app
COPY . .

# Install packages directly
RUN apt-get update && apt-get install -y curl && rm -rf /var/lib/apt/lists/*

USER app
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s CMD curl --fail http://localhost:8080/health || exit 1

CMD ["dotnet", "MyApp.dll"]
