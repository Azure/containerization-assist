import { validateDockerfileContent } from '@/validation/dockerfile-validator';

describe('validateDockerfileContent syntax handling', () => {
  it('accepts a valid ARG instruction before FROM', async () => {
    const dockerfile = `ARG NODE_VERSION=20
FROM node:\${NODE_VERSION}-alpine
WORKDIR /app
COPY . .
CMD ["node", "index.js"]`;

    const report = await validateDockerfileContent(dockerfile);

    expect(report.results.some((r) => r.ruleId === 'parse-error')).toBe(false);
    expect(report.score).toBeGreaterThan(0);
    expect(report.grade).not.toBe('F');
  });

  it('accepts multiple ARG instructions and comments before FROM', async () => {
    const dockerfile = `# base image config
ARG BASE=20
ARG VARIANT=alpine
FROM node:\${BASE}-\${VARIANT}
WORKDIR /app
CMD ["node", "index.js"]`;

    const report = await validateDockerfileContent(dockerfile);

    expect(report.results.some((r) => r.ruleId === 'parse-error')).toBe(false);
    expect(report.score).toBeGreaterThan(0);
  });

  it('still rejects a Dockerfile with no FROM instruction', async () => {
    const dockerfile = `RUN echo hi
COPY . .`;

    const report = await validateDockerfileContent(dockerfile);

    expect(report.results.some((r) => r.ruleId === 'parse-error')).toBe(true);
    expect(report.score).toBe(0);
    expect(report.grade).toBe('F');
  });

  it('produces an equivalent result for plain FROM and ARG-before-FROM', async () => {
    const plain = `FROM node:20-alpine
WORKDIR /app
COPY . .
CMD ["node", "index.js"]`;
    const argFirst = `ARG NODE_VERSION=20
FROM node:\${NODE_VERSION}-alpine
WORKDIR /app
COPY . .
CMD ["node", "index.js"]`;

    const plainReport = await validateDockerfileContent(plain);
    const argReport = await validateDockerfileContent(argFirst);

    expect(argReport.score).toBeGreaterThanOrEqual(plainReport.score - 5);
    expect(argReport.grade).not.toBe('F');
  });
});
