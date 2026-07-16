This directory would normally contain the JARs the project depends on,
checked directly into source control (the typical pre-Maven pattern).

For this fixture we only document them here — they are not actually present:

  javaee-api-6.0.jar           (Java EE 6 API; provided by the app server at runtime)
  jboss-logging-3.4.1.Final.jar
  jackson-core-2.9.10.jar
  jackson-databind-2.9.10.8.jar
  jackson-annotations-2.9.10.jar

The real-world version of this kind of repo can easily ship 30-80 JARs in lib/.
