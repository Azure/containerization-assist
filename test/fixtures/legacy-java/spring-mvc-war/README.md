# legacy-spring-mvc-war

A representative **legacy enterprise Java app** for testing containerization tooling.

## What it simulates

A typical mid-2010s internal line-of-business app:

- **Spring 4.3 MVC** (not Spring Boot — no embedded server)
- **Java 8** source/target
- **Maven** build, packaged as a **WAR**
- Designed to be deployed into an **external Apache Tomcat 8.5 / 9** servlet container
- **JSP** views (`src/main/webapp/WEB-INF/views/`)
- **JDBC** with a `DataSource` configured via Spring XML
- **log4j 1.x** (yes, the old one) for logging

## Why it's tricky to containerize

- No `Dockerfile` and no embedded server — the agent must pick a Tomcat base image
  and place the WAR under `/usr/local/tomcat/webapps/`.
- The `<finalName>` in `pom.xml` controls the WAR file name (and therefore the
  context path inside Tomcat).
- Configuration is pulled from `application.properties` baked into the WAR — the
  agent should suggest externalizing DB credentials via env vars / k8s Secrets.
- log4j 1.x is end-of-life — a security-aware agent should flag it.

## Layout

```
spring-mvc-war/
├── pom.xml
├── src/main/java/com/example/legacy/...
├── src/main/resources/
│   ├── application.properties
│   └── log4j.properties
└── src/main/webapp/
    ├── WEB-INF/
    │   ├── web.xml
    │   ├── spring-mvc-config.xml
    │   └── views/users.jsp
    └── index.jsp
```

## Build (for reference — fixture is not expected to actually build in CI)

```sh
mvn -B clean package
# → target/legacy-app.war
```
