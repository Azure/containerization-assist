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
      - In multi-module projects, the root pom.xml may be a parent that does not produce a runnable artifact. Identify the correct submodule (with its own pom.xml and src/) that builds a JAR or WAR.
      - Always COPY all required build files: at minimum pom.xml and src/, and if present, .mvn/, mvnw, configuration files, or other submodules. Avoid assuming pom.xml + src/ alone is sufficient.
      - Prefer using mvn that comes preinstalled in the docker image or installed by a previous step instead of mvnw
      - If using mvnw, COPY both the mvnw script and .mvn/ directory, and make the script executable.
      - Avoid 'mvn dependency:go-offline' unless all transitive and parent files are in the context.
      -if maven builds throw errors, try using maven with the D-skipTests flag to skip tests.
      - if mavin builds continue to fail when using a specific base version of the image (e.g if 3.9 fails try 3.6.3), try using a different base image version.
      - Prefer full 'mvn clean package' builds. Avoid partial goals unless structure is known.
      - In the build stage, use a wildcard (e.g., target/*.war) to locate the output and rename it to a known name (e.g., app.jar). Do not rely on -DfinalName.
      - In the CMD, avoid runtime wildcards or find-based commands. Reference the renamed artifact directly to ensure startup reliability.

`
