# Test: block-root-user (FAIL)
# Violation: Runs as root user
FROM mcr.microsoft.com/dotnet/aspnet:8.0

WORKDIR /app
COPY . .

USER root
EXPOSE 8080

CMD ["dotnet", "MyApp.dll"]
