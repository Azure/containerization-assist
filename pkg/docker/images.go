package docker

var ApprovedDockerImages = `
approved_images:
  - image: tomcat
    tag: "9.0"
    notes: "Tomcat should not contain the sample webapp content"
  - image: quay.io/wildfly/wildfly
    tags:
      - "latest"
      - "latest-jdk21"
      - "36.0.0.Final-jdk21"
      - "latest-jdk17"
      - "36.0.0.Final-jdk17"
  - image: jboss-eap
    tag: "7.3"
  - image: oracle/weblogic
    tag: "12.2.1.4"
  - image: ibmcom/websphere-traditional
    tag: "9.0.5.7"
  - image: glassfish
    tag: "5.1"
  - image: maven
    tags:
      - "3.6.3-jdk-8"
      - "3.8.3-openjdk-17"
      - "3.9-eclipse-temurin-8"
      - "3.9.9-eclipse-temurin-24-alpine"
    notes: >
      When using Maven in Dockerfiles:
      - Do not assume a single-module layout. Set WORKDIR to the directory containing the relevant pom.xml.
      - In multi-module projects, the root pom.xml may not produce a runnable artifact. Identify and target the correct submodule for packaging.
      - If using mvnw, COPY both the mvnw script and .mvn/ directory, and make the script executable.
      - Avoid using 'mvn dependency:go-offline' unless all required files (e.g., parent pom.xmls, .mvn) are present in the build context.
      - Prefer a full 'mvn clean package' with correct COPY structure over partial builds or dependency prefetching.
      - In the build stage, do not rely on -DfinalName to rename outputs; use a wildcard (e.g., target/*.war) to locate the artifact and rename it to a known name (e.g., app.jar).
      - In the CMD, avoid using wildcards at runtime; reference the renamed file directly to prevent startup failures.
`
