# Test: block-secrets-in-env (PASS)
# No hardcoded secrets in environment variables
FROM mcr.microsoft.com/dotnet/aspnet:8.0

WORKDIR /app

ENV NODE_ENV=production
ENV LOG_LEVEL=info
ENV PORT=8080

COPY . .

USER app
EXPOSE 8080

CMD ["dotnet", "MyApp.dll"]
