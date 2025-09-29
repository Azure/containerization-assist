import { parseAllDocuments, Document } from 'yaml';

export interface NormalizationOptions {
  addMissingDefaults?: boolean;
  fixSelectors?: boolean;
  addSecurityContext?: boolean;
  addResourceLimits?: boolean;
  addProbes?: boolean;
  standardizeLabels?: boolean;
  enforceNamespace?: string;
}

export interface NormalizationResult {
  normalized: string;
  changes: Array<{
    type: 'added' | 'modified' | 'removed';
    resource: string;
    field: string;
    description: string;
  }>;
}

export class K8sNormalizer {
  constructor(private options: NormalizationOptions = {}) {
    // Default to safe normalizations
    this.options = {
      addMissingDefaults: true,
      fixSelectors: true,
      addSecurityContext: true,
      addResourceLimits: false, // Can be resource-intensive
      addProbes: false, // Application-specific
      standardizeLabels: true,
      ...options,
    };
  }

  normalize(yamlContent: string): NormalizationResult {
    const docs = parseAllDocuments(yamlContent);
    const changes: NormalizationResult['changes'] = [];

    const normalizedDocs = docs.map((doc) => {
      const obj = doc.toJS();
      if (!obj || typeof obj !== 'object') return doc;

      const resourceName = `${obj.kind}/${obj.metadata?.name || 'unnamed'}`;
      const resourceChanges = this.normalizeResource(obj, resourceName);
      changes.push(...resourceChanges);

      return new Document(obj);
    });

    const normalized = `${normalizedDocs.map((d) => d.toString().trim()).join('\n---\n')}\n`;

    return { normalized, changes };
  }

  private normalizeResource(
    obj: Record<string, unknown>,
    resourceName: string,
  ): NormalizationResult['changes'] {
    const changes: NormalizationResult['changes'] = [];

    // Ensure metadata exists
    if (!obj.metadata) {
      obj.metadata = {};
      changes.push({
        type: 'added',
        resource: resourceName,
        field: 'metadata',
        description: 'Added missing metadata section',
      });
    }

    const metadata = obj.metadata as Record<string, unknown>;

    // Add namespace if enforced
    if (this.options.enforceNamespace && !metadata.namespace) {
      metadata.namespace = this.options.enforceNamespace;
      changes.push({
        type: 'added',
        resource: resourceName,
        field: 'metadata.namespace',
        description: `Added namespace: ${this.options.enforceNamespace}`,
      });
    }

    // Standardize labels
    if (this.options.standardizeLabels) {
      changes.push(...this.standardizeLabels(obj, resourceName));
    }

    // Resource-specific normalization
    switch (obj.kind) {
      case 'Deployment':
      case 'StatefulSet':
      case 'DaemonSet':
        changes.push(...this.normalizeWorkload(obj, resourceName));
        break;
      case 'Service':
        changes.push(...this.normalizeService(obj, resourceName));
        break;
      case 'Pod':
        changes.push(...this.normalizePodSpec(obj.spec as Record<string, unknown>, resourceName));
        break;
    }

    return changes;
  }

  private standardizeLabels(
    obj: Record<string, unknown>,
    resourceName: string,
  ): NormalizationResult['changes'] {
    const changes: NormalizationResult['changes'] = [];
    const metadata = obj.metadata as Record<string, unknown>;

    if (!metadata.labels) {
      metadata.labels = {};
      changes.push({
        type: 'added',
        resource: resourceName,
        field: 'metadata.labels',
        description: 'Added labels section',
      });
    }

    const labels = metadata.labels as Record<string, unknown>;
    const appName = metadata.name || 'app';

    if (!labels['app.kubernetes.io/name']) {
      labels['app.kubernetes.io/name'] = appName;
      changes.push({
        type: 'added',
        resource: resourceName,
        field: 'metadata.labels["app.kubernetes.io/name"]',
        description: `Added standard label: ${appName}`,
      });
    }

    return changes;
  }

  private normalizeWorkload(
    obj: Record<string, unknown>,
    resourceName: string,
  ): NormalizationResult['changes'] {
    const changes: NormalizationResult['changes'] = [];

    if (!obj.spec) {
      obj.spec = {};
      changes.push({
        type: 'added',
        resource: resourceName,
        field: 'spec',
        description: 'Added spec section',
      });
    }

    // Fix selector/template label matching
    if (this.options.fixSelectors) {
      changes.push(...this.fixSelectors(obj, resourceName));
    }

    // Normalize pod template
    if (obj.spec) {
      const spec = obj.spec as Record<string, unknown>;
      if (spec.template) {
        changes.push(
          ...this.normalizePodTemplate(spec.template as Record<string, unknown>, resourceName),
        );
      }
    }

    return changes;
  }

  private fixSelectors(
    obj: Record<string, unknown>,
    resourceName: string,
  ): NormalizationResult['changes'] {
    const changes: NormalizationResult['changes'] = [];
    const spec = obj.spec as Record<string, unknown>;

    // Ensure template has labels
    if (!spec.template) {
      spec.template = { metadata: { labels: {} }, spec: {} };
      changes.push({
        type: 'added',
        resource: resourceName,
        field: 'spec.template',
        description: 'Added pod template',
      });
    }

    const template = spec.template as Record<string, unknown>;
    if (!template.metadata) {
      template.metadata = { labels: {} };
    }

    const templateMetadata = template.metadata as Record<string, unknown>;
    if (!templateMetadata.labels) {
      templateMetadata.labels = {};
    }

    const templateLabels = templateMetadata.labels as Record<string, unknown>;

    // Add basic app label if missing
    if (!templateLabels.app && !templateLabels['app.kubernetes.io/name']) {
      const metadata = obj.metadata as Record<string, unknown> | undefined;
      templateLabels.app = metadata?.name || 'app';
      changes.push({
        type: 'added',
        resource: resourceName,
        field: 'spec.template.metadata.labels.app',
        description: `Added template label: ${templateLabels.app}`,
      });
    }

    // Sync selector with template labels
    if (!spec.selector) {
      spec.selector = { matchLabels: {} };
    }

    const selector = spec.selector as Record<string, unknown>;
    const currentSelector = JSON.stringify(selector.matchLabels || {});
    selector.matchLabels = { ...templateLabels };
    const newSelector = JSON.stringify(selector.matchLabels);

    if (currentSelector !== newSelector) {
      changes.push({
        type: 'modified',
        resource: resourceName,
        field: 'spec.selector.matchLabels',
        description: 'Synchronized selector with template labels',
      });
    }

    return changes;
  }

  private normalizePodTemplate(
    template: Record<string, unknown>,
    resourceName: string,
  ): NormalizationResult['changes'] {
    const changes: NormalizationResult['changes'] = [];

    if (!template.spec) {
      template.spec = {};
    }

    changes.push(...this.normalizePodSpec(template.spec as Record<string, unknown>, resourceName));

    return changes;
  }

  private normalizePodSpec(
    podSpec: Record<string, unknown>,
    resourceName: string,
  ): NormalizationResult['changes'] {
    const changes: NormalizationResult['changes'] = [];

    // Add security context
    if (this.options.addSecurityContext) {
      changes.push(...this.addSecurityContext(podSpec, resourceName));
    }

    // Process containers
    if (!podSpec.containers) {
      podSpec.containers = [];
    }

    const containers = podSpec.containers as Record<string, unknown>[];
    containers.forEach((container: Record<string, unknown>, index: number) => {
      changes.push(...this.normalizeContainer(container, resourceName, index));
    });

    return changes;
  }

  private addSecurityContext(
    podSpec: Record<string, unknown>,
    resourceName: string,
  ): NormalizationResult['changes'] {
    const changes: NormalizationResult['changes'] = [];

    if (!podSpec.securityContext) {
      podSpec.securityContext = {
        runAsNonRoot: true,
        fsGroup: 1000,
        seccompProfile: {
          type: 'RuntimeDefault',
        },
      };
      changes.push({
        type: 'added',
        resource: resourceName,
        field: 'spec.securityContext',
        description: 'Added pod security context',
      });
    }

    return changes;
  }

  private normalizeContainer(
    container: Record<string, unknown>,
    resourceName: string,
    index: number,
  ): NormalizationResult['changes'] {
    const changes: NormalizationResult['changes'] = [];
    const containerName = container.name || `container-${index}`;

    // Add container security context
    if (this.options.addSecurityContext && !container.securityContext) {
      container.securityContext = {
        runAsNonRoot: true,
        readOnlyRootFilesystem: true,
        allowPrivilegeEscalation: false,
        capabilities: {
          drop: ['ALL'],
        },
      };
      changes.push({
        type: 'added',
        resource: resourceName,
        field: `spec.containers[${index}].securityContext`,
        description: `Added security context for container: ${containerName}`,
      });
    }

    // Add resource limits if requested
    if (this.options.addResourceLimits && !container.resources) {
      container.resources = {
        requests: {
          cpu: '100m',
          memory: '128Mi',
        },
        limits: {
          cpu: '500m',
          memory: '512Mi',
        },
      };
      changes.push({
        type: 'added',
        resource: resourceName,
        field: `spec.containers[${index}].resources`,
        description: `Added resource limits for container: ${containerName}`,
      });
    }

    // Add probes if requested and container has ports
    const ports = container.ports as Array<Record<string, unknown>> | undefined;
    if (
      this.options.addProbes &&
      ports &&
      ports.length > 0 &&
      !container.livenessProbe &&
      !container.readinessProbe
    ) {
      const firstPort = ports[0] as Record<string, unknown>;
      const port = firstPort.containerPort;

      container.livenessProbe = {
        httpGet: { path: '/healthz', port },
        initialDelaySeconds: 30,
        periodSeconds: 10,
      };

      container.readinessProbe = {
        httpGet: { path: '/ready', port },
        initialDelaySeconds: 5,
        periodSeconds: 5,
      };

      changes.push({
        type: 'added',
        resource: resourceName,
        field: `spec.containers[${index}].livenessProbe`,
        description: `Added health probes for container: ${containerName}`,
      });
    }

    return changes;
  }

  private normalizeService(
    obj: Record<string, unknown>,
    resourceName: string,
  ): NormalizationResult['changes'] {
    const changes: NormalizationResult['changes'] = [];

    if (!obj.spec) {
      obj.spec = {};
    }

    const spec = obj.spec as Record<string, unknown>;

    // Add selector if missing
    if (this.options.fixSelectors && !spec.selector) {
      const metadata = obj.metadata as Record<string, unknown> | undefined;
      spec.selector = { app: metadata?.name || 'app' };
      changes.push({
        type: 'added',
        resource: resourceName,
        field: 'spec.selector',
        description: `Added selector: ${JSON.stringify(spec.selector)}`,
      });
    }

    return changes;
  }
}

export function createK8sNormalizer(options?: NormalizationOptions): K8sNormalizer {
  return new K8sNormalizer(options);
}
