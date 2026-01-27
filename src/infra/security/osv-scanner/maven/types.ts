/**
 * Maven-specific type definitions for POM parsing
 */

export interface Dependency {
  groupId?: string;
  artifactId?: string;
  version?: string;
}

export interface DependencySection {
  dependency?: Dependency | Dependency[];
}

export interface DependencyManagement {
  dependencies?: DependencySection;
}

export interface Parent {
  groupId?: string;
  artifactId?: string;
  version?: string;
}

export interface Properties {
  [key: string]: string;
}

export interface PomProject {
  parent?: Parent;
  properties?: Properties;
  dependencies?: DependencySection;
  dependencyManagement?: DependencyManagement;
  groupId?: string;
  artifactId?: string;
  version?: string;
}

export interface ParseResult {
  project?: PomProject;
}

/**
 * Package extracted from Docker image
 */
export interface ExtractedPackage {
  name: string; // groupId:artifactId for Maven, package name for Debian/Alpine
  version: string;
  ecosystem: string; // Maven, Debian, Debian:9, Alpine, Alpine:v3.18, etc.
  path?: string;
  metadata?: Record<string, unknown>;
}
