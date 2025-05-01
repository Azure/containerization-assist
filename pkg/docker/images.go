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
      - Do not assume a single-module layout. Ensure the WORKDIR matches the directory containing the relevant pom.xml.
      - If using mvnw, be sure to COPY both the mvnw script and the .mvn/ directory, and make the script executable.
      - Avoid calling 'mvn dependency:go-offline' unless all required files (e.g., parent pom.xmls, .mvn directory) are present in the build context.
      - Prefer a full 'mvn clean package' with proper COPY instructions over partial builds that assume internal structure.
      - Avoid wildcard-based CMDs that rely on artifact names like '*-SNAPSHOT.jar'. Instead, rename the output to a known file (e.g., app.jar) during the build stage.
`
