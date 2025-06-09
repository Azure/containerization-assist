FROM maven:3.9-eclipse-temurin-17 AS build

WORKDIR /app
COPY . .

RUN mvn clean package -DskipTests -B

FROM tomcat:9.0-jdk17-temurin

# Automatically detect any WAR file and deploy it as ROOT.war
COPY --from=build /app/target/*.war /usr/local/tomcat/webapps/ROOT.war

EXPOSE 8080

CMD ["catalina.sh", "run"]
