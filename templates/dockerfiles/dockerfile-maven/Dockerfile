# ---------- Stage 1: Build ----------
FROM maven:3 AS BUILD

# Set working directory
WORKDIR /usr/src/app

# Copy source code
COPY . .
RUN mvn --batch-mode -f /usr/src/app/pom.xml clean package
RUN find /usr/src/app/target -type f -name '*-SNAPSHOT.jar' -exec cp {} /usr/src/app/app.jar \;
# ---------- Stage 2: Runtime ----------
FROM eclipse-temurin:21-jre

# Set environment and expose port
ENV PORT 80
EXPOSE 80

# Copy build artifacts  from the build stage
COPY --from=BUILD /usr/src/app/app.jar /opt/app.jar
WORKDIR /opt

CMD ["java", "-jar", "app.jar"]
