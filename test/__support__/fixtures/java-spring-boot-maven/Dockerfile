# Multi-stage build for Java
FROM maven:3-amazoncorretto-17 AS builder
COPY pom.xml .
RUN mvn dependency:go-offline
RUN mvn package -DskipTests

COPY --from=builder /build/target/*.jar app.jar
# Copy JAR file
COPY target/*.jar app.jar

# Security hardening
RUN addgroup -g 1001 -S appgroup && adduser -u 1001 -S appuser -G appgroup
USER appuser

# Expose application port
EXPOSE 8080

# Start application
CMD ["java", "-jar", "app.jar"]
