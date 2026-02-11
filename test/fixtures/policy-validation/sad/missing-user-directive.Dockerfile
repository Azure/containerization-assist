# Test: require-user-directive (WARN)
# Warning: Missing USER directive
FROM mcr.microsoft.com/dotnet/aspnet:8.0

WORKDIR /app
COPY . .

EXPOSE 8080

CMD ["dotnet", "MyApp.dll"]
