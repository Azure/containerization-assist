# spring-boot-rest-api

Realistic legacy-Java fixture for the agent-eval framework.

A small **Spring Boot 2.7** REST API on **JDK 17**, built with Maven into a
runnable fat-jar via `spring-boot-maven-plugin`. This is the bread-and-butter
"modern legacy" enterprise scenario the containerization tooling targets:

- Spring Boot 2.7.x (still widely deployed)
- JDK 17 (matches MCR `mcr.microsoft.com/openjdk/jdk:17-azurelinux`)
- Single REST endpoint at `/api/orders` — enough surface area for the agent
  to recognize a Spring web app without ceremony
- No external services, no databases — keeps the docker build fast in CI

Compared to the other fixtures:

- `spring-mvc-war` exercises the **JDK-8 + WAR + servlet container** path,
  which forces the agent into legacy territory (and is where the
  knowledge pack's MCR-tag gap is most visible).
- `ejb-ant-monolith` exercises the **Ant + EJB** path, which needs a full
  Java EE app server — none of the modes know that.
- This fixture exercises the **happy path**: the agent should produce a
  build-clean Dockerfile on a supported MCR base image with no special
  guidance beyond what the skill / MCP tool already provides.
