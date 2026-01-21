# Test: require-workdir (WARN)
# Uses cd instead of WORKDIR
FROM mcr.microsoft.com/dotnet/aspnet:8.0

COPY . /app
RUN cd /app && echo "setup"

USER app
EXPOSE 8080

CMD ["dotnet", "/app/MyApp.dll"]
