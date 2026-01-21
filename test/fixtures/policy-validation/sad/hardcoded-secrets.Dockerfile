# Test: block-secrets-in-env (FAIL)
# Violation: Hardcoded secrets in environment variables
FROM mcr.microsoft.com/dotnet/aspnet:8.0

WORKDIR /app

ENV NODE_ENV=production
ENV API_KEY=sk_live_abc123456789
ENV DB_PASSWORD=supersecret123
ENV AUTH_TOKEN=bearer_xyz987654321

COPY . .

USER app
EXPOSE 8080

CMD ["dotnet", "MyApp.dll"]
