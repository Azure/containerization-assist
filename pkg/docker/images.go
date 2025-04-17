package docker

var ApprovedDockerImages = `
approved_images:
  - image: tomcat
    tag: "9.0"
    notes: "Tomcat should not contain the sample webapp content"
  - image: jboss/wildfly
    tag: "latest"
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
`
