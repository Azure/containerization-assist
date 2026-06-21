# Legacy Java fixture repos

Two representative "legacy" Java backends used to evaluate how well
containerization-assist (and bare LLM agents) handle real-world enterprise apps.

These are **not** intended to actually compile in CI — they exist so the analyzer,
Dockerfile generator, and k8s manifest generator have realistic-shaped input to
chew on. Source files are minimal but the build manifests, descriptors, packaging
metadata, and config files are all genuine legacy patterns.

| Fixture | Era | Build | Runtime target | Why it's interesting |
|---|---|---|---|---|
| [`spring-mvc-war/`](spring-mvc-war/) | Mid-2010s | Maven (WAR) | External Tomcat 8.5/9 | No embedded server. JSP views. log4j 1.x (EOL). Credentials in `application.properties`. |
| [`ejb-ant-monolith/`](ejb-ant-monolith/) | Late-2000s / Early-2010s | Apache Ant | WildFly / JBoss EAP | No `pom.xml` — JARs in `lib/`. EJB 3.1 + JAX-RS. JNDI-bound `DataSource`. |

## Using with the agent-eval harness

The agent-eval CLI ([`test/agent-eval/cli.ts`](../../agent-eval/cli.ts)) accepts any
fixture directory:

```sh
# Baseline LLM, no skills, no MCP
tsx test/agent-eval/cli.ts run \
  --fixture test/fixtures/legacy-java/spring-mvc-war \
  --mode baseline \
  --model github-models:gpt-4o-mini

# With containerization-assist MCP tools available
tsx test/agent-eval/cli.ts run \
  --fixture test/fixtures/legacy-java/ejb-ant-monolith \
  --mode mcp \
  --model github-models:gpt-4o-mini
```

The CLI copies the fixture into a temp directory before running, so the agent
can write `Dockerfile` / `k8s/*.yaml` artifacts without polluting source.

## What a "good" containerization looks like for each

### `spring-mvc-war`

- Multi-stage Dockerfile: `maven:3.9-eclipse-temurin-8` build stage →
  `tomcat:9-jre8` runtime stage with the WAR copied to
  `/usr/local/tomcat/webapps/ROOT.war` (or `legacy-app.war` to preserve the path).
- DB credentials sourced from env vars, not the baked-in
  `application.properties`. Spring's `${db.password}` placeholders should be
  overridable via `JAVA_OPTS=-Dspring.config.location=...` or, more pragmatically,
  a sidecar `setenv.sh`.
- Health check hitting `/` or a small actuator-style endpoint.
- Non-root user (`tomcat`).
- Flag log4j 1.x as a remediation item.

### `ejb-ant-monolith`

- Build stage needs **Ant**, not Maven — e.g. an image with
  `eclipse-temurin:8-jdk` + `apt-get install ant`, or one of the
  `gradle/gradle:jdk8` images repurposed.
- Runtime stage must be a Java EE app server — `quay.io/wildfly/wildfly:26.1.3.Final-jdk11`
  is a good default. Plain Tomcat will not work (EJBs / JNDI DS lookups).
- The DataSource binding (`java:jboss/datasources/InvoicesDS`) needs to be
  registered with the WildFly CLI on startup; agent should generate either a
  `*-cli.cli` script or a small `entrypoint.sh` wrapper.
- WAR path: `/opt/jboss/wildfly/standalone/deployments/invoices.war`.
