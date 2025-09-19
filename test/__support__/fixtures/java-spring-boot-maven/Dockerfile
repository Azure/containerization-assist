# Multi-stage build for TypeScript (Node.js), Spring Boot (Java), ASP.NET (.NET)
# Build TypeScript app
FROM node:20-alpine AS ts-builder
WORKDIR /app/ts
COPY package*.json ./
RUN npm ci --only=production
COPY . .
RUN npm run build

# Build Spring Boot app
FROM maven:3-eclipse-temurin-17 AS java-builder
WORKDIR /app/java
COPY pom.xml ./
RUN mvn dependency:go-offline -B
COPY src ./src
RUN mvn package -DskipTests

# Build ASP.NET app
FROM mcr.microsoft.com/dotnet/sdk:6.0 AS dotnet-builder
WORKDIR /app/dotnet
COPY BookService.csproj ./
RUN dotnet restore BookService.csproj
COPY . .
RUN dotnet publish BookService.csproj -c Release -o out

# Runtime stage
FROM node:20-alpine AS runtime
WORKDIR /app
RUN addgroup -g 1001 -S appuser && adduser -u 1001 -S appuser -G appuser
# Copy TypeScript build
COPY --from=ts-builder --chown=appuser:appuser /app/ts/dist ./ts/dist
COPY --from=ts-builder --chown=appuser:appuser /app/ts/package*.json ./ts/
# Copy Java jar
COPY --from=java-builder --chown=appuser:appuser /app/java/target/*.jar ./java/app.jar
# Copy ASP.NET publish
COPY --from=dotnet-builder --chown=appuser:appuser /app/dotnet/out ./dotnet/out
USER appuser
EXPOSE 3000 8080 5000
HEALTHCHECK --interval=30s --timeout=3s --start-period=30s --retries=3 CMD wget --no-verbose --tries=1 --spider http://localhost:3000/health || wget --no-verbose --tries=1 --spider http://localhost:8080/actuator/health || wget --no-verbose --tries=1 --spider http://localhost:5000/health || exit 1
CMD ["sh", "-c", "node ts/dist/{{entryPoint}} & java -XX:+UseContainerSupport -XX:MaxRAMPercentage=75 -jar java/app.jar & dotnet dotnet/out/BookService.dll"]