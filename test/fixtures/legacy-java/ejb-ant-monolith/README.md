# legacy-ejb-ant-monolith

A representative **deep-legacy enterprise Java app** for testing containerization tooling.

## What it simulates

The kind of pre-Maven enterprise app that's still running in some banks and insurers:

- **Java EE 6** — uses **EJB 3.1** session beans + **JAX-RS** servlets
- **Java 8** source/target (often built originally for Java 6/7 and dragged forward)
- **Apache Ant** build (`build.xml`) — no `pom.xml`, no `build.gradle`
- Dependency JARs checked into `lib/` (the original sin of pre-Maven Java)
- Packaged as a **WAR** but with EJB beans inside — designed for a full Java EE app server
  like **WildFly 10+ / JBoss EAP 7** (not plain Tomcat)
- Persistence configured via JNDI lookup of an app-server-managed `DataSource`
  (`java:jboss/datasources/InvoicesDS`)

## Why it's especially tricky to containerize

- An agent that auto-detects "Java" and reaches for a Maven Tomcat workflow will fail —
  this needs a **WildFly/JBoss base image** and an `ant dist` invocation, not `mvn package`.
- There is no machine-readable dependency manifest. The `lib/` directory is the manifest.
- The DataSource is a JNDI binding — the agent should suggest configuring it via the app
  server's CLI on container start, or driving it from env vars at deploy time.
- `ant` itself isn't installed in most build base images.

## Layout

```
ejb-ant-monolith/
├── build.xml
├── lib/
│   └── README.txt        ← placeholder; real repos have *.jar checked in
├── src/com/example/legacy/
│   ├── ejb/InvoiceBean.java
│   ├── rest/InvoiceResource.java
│   ├── rest/JaxRsApplication.java
│   └── model/Invoice.java
└── web/WEB-INF/
    ├── web.xml
    ├── beans.xml
    └── jboss-web.xml
```

## Build (for reference — fixture is not expected to actually build in CI)

```sh
ant dist
# → dist/invoices.war
```
