package dockerfile

// JavaTemplate provides the Dockerfile template for Java applications
const JavaTemplate = `# Build stage
FROM maven:3.9-eclipse-temurin-{{.LanguageVersion}} AS builder
WORKDIR /app
COPY . .

# Build the application
RUN if [ -f "mvnw" ]; then \
      chmod +x mvnw && ./mvnw clean package -DskipTests; \
    elif [ -f "gradlew" ]; then \
      chmod +x gradlew && ./gradlew build -x test; \
    elif [ -f "pom.xml" ]; then \
      mvn clean package -DskipTests; \
    elif [ -f "build.gradle" ] || [ -f "build.gradle.kts" ]; then \
      gradle build -x test; \
    else \
      echo "No build file found" && exit 1; \
    fi

{{if .IsServlet -}}
# Runtime stage - Tomcat for servlet applications
FROM tomcat:10-jre{{.LanguageVersion}}-temurin-jammy
# Remove default webapps
RUN rm -rf /usr/local/tomcat/webapps/*
# Copy WAR file to Tomcat webapps as ROOT for root context
COPY --from=builder /app/target/*.war /usr/local/tomcat/webapps/ROOT.war
{{if .Port -}}
EXPOSE {{.Port}}
{{else -}}
EXPOSE 8080
{{end -}}
# Start Tomcat
CMD ["catalina.sh", "run"]
{{else -}}
# Runtime stage
FROM eclipse-temurin:{{.LanguageVersion}}-jre-alpine
WORKDIR /app

# Copy built artifacts from builder stage
# Use a shell script to find and copy the built artifact
RUN --mount=from=builder,source=/app,target=/build \
    find /build -name '*.jar' -not -name '*-sources.jar' -not -name '*-javadoc.jar' | head -1 | xargs -I {} cp {} /app/app.jar

{{if .Port -}}
EXPOSE {{.Port}}
{{else -}}
EXPOSE 8080
{{end -}}
# Run the application
ENTRYPOINT ["java", "-jar", "app.jar"]
{{end -}}
`
