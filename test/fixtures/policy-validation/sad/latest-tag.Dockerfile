# Test: block-latest-tag (FAIL)
# Violation: Uses :latest tag
FROM mcr.microsoft.com/dotnet/aspnet:latest

WORKDIR /app
COPY . .

USER app
EXPOSE 8080

CMD ["dotnet", "MyApp.dll"]
