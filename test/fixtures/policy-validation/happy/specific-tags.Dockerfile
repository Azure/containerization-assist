# Test: block-latest-tag (PASS)
# Uses specific version tag, not :latest
FROM mcr.microsoft.com/dotnet/aspnet:8.0

WORKDIR /app
COPY . .

USER app
EXPOSE 8080

CMD ["dotnet", "MyApp.dll"]
