/**
 * Property Resolution
 * Local property resolution for Maven POM files (no remote fetching).
 */

import type { Logger } from 'pino';
import type { PomProject, Dependency } from './types';

/** Flatten nested property objects from xml2js (e.g., {spring: {version: '2.7.0'}} -> {'spring.version': '2.7.0'}) */
function flattenProperties(obj: Record<string, unknown>, prefix: string = ''): Map<string, string> {
  const result = new Map<string, string>();

  for (const [key, value] of Object.entries(obj)) {
    const fullKey = prefix ? `${prefix}.${key}` : key;

    if (typeof value === 'string') {
      result.set(fullKey, value);
    } else if (typeof value === 'object' && value !== null && !Array.isArray(value)) {
      // Recursively flatten nested objects
      const nested = flattenProperties(value as Record<string, unknown>, fullKey);
      for (const [nestedKey, nestedValue] of nested) {
        result.set(nestedKey, nestedValue);
      }
    }
  }

  return result;
}

/** Extract local properties from current POM only (no parent fetching) */
export function extractLocalProperties(project: PomProject, logger: Logger): Map<string, string> {
  const properties = new Map<string, string>();

  if (project.properties) {
    const flattened = flattenProperties(project.properties);
    for (const [key, value] of flattened) {
      properties.set(key, value);
    }
  }

  // Log warning if parent POM exists (we won't resolve it)
  if (project.parent?.groupId && project.parent?.artifactId && project.parent?.version) {
    const parentKey = `${project.parent.groupId}:${project.parent.artifactId}:${project.parent.version}`;
    logger.debug(
      { parent: parentKey },
      'Parent POM detected but remote fetching is disabled - using local properties only',
    );
  }

  return properties;
}

/** Extract local dependency management from current POM only (no parent fetching) */
export function extractLocalDependencyManagement(project: PomProject): Map<string, string> {
  const depMgmt = new Map<string, string>();

  const projectDep = project.dependencyManagement?.dependencies?.dependency;
  if (projectDep) {
    let deps: Dependency[] = [];
    if (Array.isArray(projectDep)) {
      deps = projectDep;
    } else {
      deps = [projectDep];
    }

    for (const dep of deps) {
      if (dep.groupId && dep.artifactId && dep.version) {
        const key = `${dep.groupId}:${dep.artifactId}`;
        depMgmt.set(key, dep.version);
      }
    }
  }

  return depMgmt;
}

/** Resolve property placeholders like ${project.version} or ${spring.version} */
export function resolvePropertyValue(
  value: string,
  properties: Map<string, string>,
  project: PomProject,
): string {
  let resolved = value;

  // Built-in project properties
  resolved = resolved.replace(/\$\{project\.version\}/g, project.version || '');
  resolved = resolved.replace(/\$\{project\.groupId\}/g, project.groupId || '');
  resolved = resolved.replace(/\$\{project\.artifactId\}/g, project.artifactId || '');
  resolved = resolved.replace(/\$\{pom\.version\}/g, project.version || '');
  resolved = resolved.replace(/\$\{pom\.groupId\}/g, project.groupId || '');
  resolved = resolved.replace(/\$\{pom\.artifactId\}/g, project.artifactId || '');

  // Custom properties
  const propertyPattern = /\$\{([^}]+)\}/g;
  let match;

  while ((match = propertyPattern.exec(resolved)) !== null) {
    const propName = match[1];
    if (propName) {
      const propValue = properties.get(propName);
      if (propValue) {
        resolved = resolved.replace(match[0], propValue);
      }
    }
  }

  return resolved;
}
