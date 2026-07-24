/**
 * Local pre-flight for the Maven-mirror build path (Track B).
 *
 * The ADO agent-eval pipeline is slow and costly, so any change to how the eval
 * builds Maven fixtures MUST be proven end-to-end locally first — with a real
 * `docker build` that resolves dependencies through the Azure Artifacts feed —
 * not just with the string-transform unit checks. (A shell `&&` env-precedence
 * bug once passed the unit checks but only failed inside a real container.)
 *
 * Usage (requires docker + reachability to the feed):
 *   export AGENT_EVAL_MAVEN_MIRROR=1
 *   export AGENT_EVAL_MAVEN_TOKEN="$(az account get-access-token \
 *     --resource 499b84ac-1321-427f-aa17-267ca6975798 --query accessToken -o tsv)"
 *   npx tsx test/agent-eval/maven-ci-preflight.ts [fixtureDir]
 *
 * Exit 0 = the fixture built through the feed; non-zero = failure.
 */
import { promises as fs } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { spawnSync } from 'node:child_process';
import { prepareMavenBuild } from './maven-ci.js';

const DEFAULT_FIXTURE = 'test/fixtures/legacy-java/spring-boot-rest-api';

// A representative agent-style Dockerfile: multi-stage build, and the exact
// `chmod && ./mvnw` chain whose credential handling is easy to get wrong.
const DOCKERFILE = `FROM mcr.microsoft.com/openjdk/jdk:17-azurelinux AS build
WORKDIR /app
COPY . .
RUN chmod +x ./mvnw && ./mvnw -B clean package -DskipTests
FROM mcr.microsoft.com/openjdk/jdk:17-distroless
WORKDIR /app
COPY --from=build /app/target/*.jar /app/app.jar
ENTRYPOINT ["java", "-jar", "/app/app.jar"]
`;

async function main(): Promise<void> {
  const fixture = process.argv[2] ?? DEFAULT_FIXTURE;
  const work = await fs.mkdtemp(join(tmpdir(), 'maven-preflight-'));
  await fs.cp(fixture, work, { recursive: true });
  await fs.writeFile(join(work, 'Dockerfile'), DOCKERFILE, 'utf8');

  const prep = await prepareMavenBuild(work);
  if (!prep) {
    console.error(
      'FAIL: prepareMavenBuild returned null. Set AGENT_EVAL_MAVEN_MIRROR=1 and ' +
        'AGENT_EVAL_MAVEN_TOKEN (or SYSTEM_ACCESSTOKEN), and ensure the fixture has a pom.xml.',
    );
    process.exit(2);
  }

  const rewritten = await fs.readFile(join(work, 'Dockerfile'), 'utf8');
  console.log('--- transformed mvn/mvnw RUN line ---');
  console.log(rewritten.split('\n').find((l) => /mvnw?\b/.test(l)) ?? '(none)');

  const args = ['build', ...prep.secretArgs, '-t', 'maven-ci-preflight:check', work];
  console.log(`--- docker ${args.join(' ')} ---`);
  const res = spawnSync('docker', args, {
    stdio: 'inherit',
    env: { ...process.env, DOCKER_BUILDKIT: '1' },
  });

  await prep.cleanup();
  await fs.rm(work, { recursive: true, force: true });

  if (res.status === 0) {
    console.log('\n==== PREFLIGHT PASS: fixture built through the Maven feed ====');
    process.exit(0);
  }
  console.error(`\n==== PREFLIGHT FAIL: docker build exit ${res.status} ====`);
  process.exit(1);
}

main().catch((e) => {
  console.error('preflight harness error:', e);
  process.exit(3);
});
