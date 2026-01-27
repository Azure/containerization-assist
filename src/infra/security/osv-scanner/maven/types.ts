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
 * Package extracted from Docker image (Maven only)
 */
export interface ExtractedPackage {
  name: string; // groupId:artifactId format
  version: string;
  ecosystem: 'Maven';
  path?: string;
}
