/**
 * Java Spring Boot Basic Repository Fixture
 * Simple Spring Boot application for testing
 */

export const javaSpringBootBasicRepository = {
  'pom.xml': `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>
    
    <parent>
        <groupId>org.springframework.boot</groupId>
        <artifactId>spring-boot-starter-parent</artifactId>
        <version>3.1.5</version>
        <relativePath/>
    </parent>
    
    <groupId>com.example</groupId>
    <artifactId>java-springboot-basic</artifactId>
    <version>1.0.0</version>
    <name>java-springboot-basic</name>
    <description>Basic Spring Boot application</description>
    
    <properties>
        <java.version>17</java.version>
    </properties>
    
    <dependencies>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-web</artifactId>
        </dependency>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-actuator</artifactId>
        </dependency>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-test</artifactId>
            <scope>test</scope>
        </dependency>
    </dependencies>
    
    <build>
        <plugins>
            <plugin>
                <groupId>org.springframework.boot</groupId>
                <artifactId>spring-boot-maven-plugin</artifactId>
            </plugin>
        </plugins>
    </build>
</project>`,
  'src/main/java/com/example/Application.java': `package com.example;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

@SpringBootApplication
public class Application {
    public static void main(String[] args) {
        SpringApplication.run(Application.class, args);
    }
}`,
  'src/main/java/com/example/controller/HelloController.java': `package com.example.controller;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;
import java.time.LocalDateTime;
import java.util.HashMap;
import java.util.Map;

@RestController
public class HelloController {
    
    @GetMapping("/")
    public Map<String, Object> hello() {
        Map<String, Object> response = new HashMap<>();
        response.put("message", "Hello World!");
        response.put("timestamp", LocalDateTime.now());
        return response;
    }
    
    @GetMapping("/health")
    public Map<String, String> health() {
        Map<String, String> response = new HashMap<>();
        response.put("status", "healthy");
        response.put("version", "1.0.0");
        return response;
    }
}`,
  'src/main/resources/application.properties': `server.port=8080
management.endpoints.web.exposure.include=health,info
management.endpoint.health.show-details=when-authorized`,
  'README.md': `# Java Spring Boot Basic

A simple Spring Boot application for testing containerization.

## Running the application

\`\`\`bash
./mvnw spring-boot:run
\`\`\`

The server will start on port 8080.`,
  '.gitignore': `target/
!.mvn/wrapper/maven-wrapper.jar
!**/src/main/**/target/
!**/src/test/**/target/

### STS ###
.apt_generated
.classpath
.factorypath
.project
.settings
.springBeans
.sts4-cache

### IntelliJ IDEA ###
.idea
*.iws
*.iml
*.ipr

### NetBeans ###
/nbproject/private/
/nbbuild/
/dist/
/nbdist/
/.nb-gradle/
build/
!**/src/main/**/build/
!**/src/test/**/build/`,
};

export const expectedJavaSpringBootAnalysis = {
  projectType: 'java',
  packageManager: 'maven',
  buildTool: 'maven',
  dependencies: ['spring-boot-starter-web', 'spring-boot-starter-actuator'],
  devDependencies: ['spring-boot-starter-test'],
  buildFile: 'pom.xml',
  ports: [8080],
  javaVersion: '17',
  springBootVersion: '3.1.5',
  hasDockerfile: false,
  hasTests: true,
  mainClass: 'com.example.Application',
};

export const expectedJavaSpringBootDockerfile = `FROM openjdk:17-jdk-slim

WORKDIR /app

COPY pom.xml ./
COPY .mvn .mvn
COPY mvnw ./
RUN chmod +x mvnw && ./mvnw dependency:go-offline

COPY src src
RUN ./mvnw clean package -DskipTests

EXPOSE 8080

RUN addgroup --system appgroup && adduser --system --ingroup appgroup appuser
USER appuser

CMD ["java", "-jar", "target/java-springboot-basic-1.0.0.jar"]`;

export {};