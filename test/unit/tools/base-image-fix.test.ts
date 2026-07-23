import { fixBaseImages } from '@/tools/fix-dockerfile/base-image-fix';
import { mcrRefExists } from '@/validation/mcr-registry';

async function mcrReachable(): Promise<boolean> {
  return (await mcrRefExists('mcr.microsoft.com/openjdk/jdk:17-azurelinux')) !== null;
}

describe('fixBaseImages', () => {
  it('returns no change when there are no FROM lines', async () => {
    const res = await fixBaseImages('# just a comment\nRUN echo hi\n');
    expect(res.substitutions).toHaveLength(0);
    expect(res.fixedDockerfile).toBeNull();
  });

  it('substitutes a non-MCR image that has a verified MCR equivalent, preserving structure', async () => {
    const df = [
      '# syntax=docker/dockerfile:1',
      '# build stage',
      'FROM --platform=$BUILDPLATFORM eclipse-temurin:17-jdk AS build',
      'WORKDIR /app',
      'COPY . .',
      'RUN ./mvnw package',
      '',
      'FROM eclipse-temurin:17-jre AS runtime',
      'COPY --from=build /app/app.jar /app.jar',
      'ENTRYPOINT ["java","-jar","/app.jar"]',
      '',
    ].join('\n');

    const res = await fixBaseImages(df);
    expect(res.substitutions.map((s) => s.replacement)).toEqual([
      'mcr.microsoft.com/openjdk/jdk:17-azurelinux',
      'mcr.microsoft.com/openjdk/jdk:17-distroless',
    ]);
    // Build stage → build image; final stage → runtime image.
    expect(res.substitutions[0]!.stage).toBe('build');
    expect(res.substitutions[1]!.stage).toBe('runtime');

    const out = res.fixedDockerfile!;
    // Preserved: syntax directive, comments, --platform, AS aliases, other lines.
    expect(out).toContain('# syntax=docker/dockerfile:1');
    expect(out).toContain('# build stage');
    expect(out).toContain(
      'FROM --platform=$BUILDPLATFORM mcr.microsoft.com/openjdk/jdk:17-azurelinux AS build',
    );
    expect(out).toContain('FROM mcr.microsoft.com/openjdk/jdk:17-distroless AS runtime');
    expect(out).toContain('COPY --from=build /app/app.jar /app.jar');
    // Line count unchanged (surgical, not reconstructed).
    expect(out.split('\n')).toHaveLength(df.split('\n').length);
  });

  it('leaves a non-MCR image with no MCR equivalent untouched', async () => {
    const df = 'FROM golang:1.21-alpine AS build\nRUN go build\n';
    const res = await fixBaseImages(df);
    expect(res.substitutions).toHaveLength(0);
    expect(res.fixedDockerfile).toBeNull();
  });

  it('leaves a real MCR image untouched (live)', async () => {
    if (!(await mcrReachable())) return;
    const df = 'FROM mcr.microsoft.com/openjdk/jdk:17-azurelinux\n';
    const res = await fixBaseImages(df);
    expect(res.substitutions).toHaveLength(0);
  }, 30000);

  it('replaces a hallucinated MCR tag with a verified one (live)', async () => {
    if (!(await mcrReachable())) return;
    const df = 'FROM mcr.microsoft.com/java/jre:8u372-zulu-ubuntu\n';
    const res = await fixBaseImages(df);
    expect(res.substitutions).toHaveLength(1);
    expect(res.substitutions[0]!.reason).toBe('hallucinated-mcr-tag');
    expect(res.substitutions[0]!.replacement).toBe('mcr.microsoft.com/openjdk/jdk:8-azurelinux');
  }, 30000);
});
