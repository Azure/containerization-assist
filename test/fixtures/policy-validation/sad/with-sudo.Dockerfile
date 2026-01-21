# Test: avoid-sudo (WARN)
# Uses sudo in RUN commands
FROM mcr.microsoft.com/dotnet/aspnet:8.0

WORKDIR /app
COPY . .

# Uses sudo (not recommended in containers)
RUN sudo apt-get update && sudo apt-get install -y curl

USER app
EXPOSE 8080

CMD ["dotnet", "MyApp.dll"]
