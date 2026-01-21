# Test: require-healthcheck (WARN)
# Warning: Missing HEALTHCHECK directive
FROM mcr.microsoft.com/dotnet/aspnet:8.0

WORKDIR /app
COPY . .

USER app
EXPOSE 8080

CMD ["dotnet", "MyApp.dll"]
