/**
 * Kubernetes API and resource type definitions
 * Provides type safety for Kubernetes API interactions and manifest operations
 */

// Core Kubernetes API object structure
export interface K8sAPIObject {
  apiVersion: string;
  kind: string;
  metadata: K8sMetadata;
  spec?: Record<string, unknown>;
  status?: Record<string, unknown>;
}

export interface K8sMetadata {
  name: string;
  namespace?: string;
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
  uid?: string;
  resourceVersion?: string;
  creationTimestamp?: string | Date;
  deletionTimestamp?: string;
  finalizers?: string[];
  ownerReferences?: K8sOwnerReference[];
  generateName?: string;
}

export interface K8sOwnerReference {
  apiVersion: string;
  kind: string;
  name: string;
  uid: string;
  controller?: boolean;
  blockOwnerDeletion?: boolean;
}

// Enhanced K8sManifest that extends the basic one from consolidated-types
export interface K8sManifest extends K8sAPIObject {
  metadata: K8sMetadata;
}

// Specific resource types
export interface K8sDeployment extends K8sManifest {
  kind: 'Deployment';
  spec: {
    replicas?: number;
    selector: {
      matchLabels: Record<string, string>;
    };
    template: {
      metadata: {
        labels: Record<string, string>;
        annotations?: Record<string, string>;
      };
      spec: K8sPodSpec;
    };
    strategy?: {
      type: 'RollingUpdate' | 'Recreate';
      rollingUpdate?: {
        maxUnavailable?: number | string;
        maxSurge?: number | string;
      };
    };
    minReadySeconds?: number;
    revisionHistoryLimit?: number;
    paused?: boolean;
    progressDeadlineSeconds?: number;
  };
  status?: {
    replicas?: number;
    updatedReplicas?: number;
    readyReplicas?: number;
    availableReplicas?: number;
    unavailableReplicas?: number;
    observedGeneration?: number;
    conditions?: K8sCondition[];
  };
}

export interface K8sService extends K8sManifest {
  kind: 'Service';
  spec: {
    selector?: Record<string, string>;
    ports: Array<{
      name?: string;
      protocol?: 'TCP' | 'UDP' | 'SCTP';
      port: number;
      targetPort?: number | string;
      nodePort?: number;
    }>;
    type?: 'ClusterIP' | 'NodePort' | 'LoadBalancer' | 'ExternalName';
    clusterIP?: string;
    clusterIPs?: string[];
    externalIPs?: string[];
    loadBalancerIP?: string;
    loadBalancerSourceRanges?: string[];
    externalName?: string;
    sessionAffinity?: 'None' | 'ClientIP';
    sessionAffinityConfig?: {
      clientIP?: {
        timeoutSeconds?: number;
      };
    };
  };
  status?: {
    loadBalancer?: {
      ingress?: Array<{
        ip?: string;
        hostname?: string;
      }>;
    };
  };
}

export interface K8sIngress extends K8sManifest {
  kind: 'Ingress';
  spec: {
    ingressClassName?: string;
    defaultBackend?: K8sIngressBackend;
    tls?: Array<{
      hosts?: string[];
      secretName?: string;
    }>;
    rules?: Array<{
      host?: string;
      http?: {
        paths: Array<{
          path?: string;
          pathType: 'Exact' | 'Prefix' | 'ImplementationSpecific';
          backend: K8sIngressBackend;
        }>;
      };
    }>;
  };
}

export interface K8sIngressBackend {
  service?: {
    name: string;
    port: {
      number?: number;
      name?: string;
    };
  };
  resource?: {
    apiGroup?: string;
    kind: string;
    name: string;
  };
}

export interface K8sConfigMap extends K8sManifest {
  kind: 'ConfigMap';
  data?: Record<string, string>;
  binaryData?: Record<string, string>;
}

export interface K8sSecret extends K8sManifest {
  kind: 'Secret';
  type?: string;
  data?: Record<string, string>;
  stringData?: Record<string, string>;
}

export interface K8sPersistentVolumeClaim extends K8sManifest {
  kind: 'PersistentVolumeClaim';
  spec: {
    accessModes: string[];
    resources: {
      requests: {
        storage: string;
      };
    };
    storageClassName?: string;
    selector?: {
      matchLabels?: Record<string, string>;
    };
    volumeName?: string;
    volumeMode?: 'Filesystem' | 'Block';
  };
}

// Common structures
export interface K8sPodSpec {
  containers: K8sContainer[];
  initContainers?: K8sContainer[];
  restartPolicy?: 'Always' | 'OnFailure' | 'Never';
  terminationGracePeriodSeconds?: number;
  activeDeadlineSeconds?: number;
  dnsPolicy?: 'ClusterFirst' | 'ClusterFirstWithHostNet' | 'Default' | 'None';
  nodeSelector?: Record<string, string>;
  serviceAccountName?: string;
  serviceAccount?: string;
  automountServiceAccountToken?: boolean;
  nodeName?: string;
  hostNetwork?: boolean;
  hostPID?: boolean;
  hostIPC?: boolean;
  securityContext?: K8sPodSecurityContext;
  imagePullSecrets?: Array<{ name: string }>;
  hostname?: string;
  subdomain?: string;
  affinity?: K8sAffinity;
  schedulerName?: string;
  tolerations?: K8sToleration[];
  hostAliases?: Array<{
    ip: string;
    hostnames: string[];
  }>;
  priorityClassName?: string;
  priority?: number;
  dnsConfig?: K8sPodDNSConfig;
  volumes?: K8sVolume[];
}

export interface K8sContainer {
  name: string;
  image: string;
  command?: string[];
  args?: string[];
  workingDir?: string;
  ports?: Array<{
    name?: string;
    containerPort: number;
    protocol?: 'TCP' | 'UDP' | 'SCTP';
  }>;
  env?: Array<{
    name: string;
    value?: string;
    valueFrom?: {
      fieldRef?: {
        apiVersion?: string;
        fieldPath: string;
      };
      resourceFieldRef?: {
        containerName?: string;
        resource: string;
        divisor?: string;
      };
      configMapKeyRef?: {
        name: string;
        key: string;
        optional?: boolean;
      };
      secretKeyRef?: {
        name: string;
        key: string;
        optional?: boolean;
      };
    };
  }>;
  resources?: {
    limits?: Record<string, string>;
    requests?: Record<string, string>;
  };
  volumeMounts?: Array<{
    name: string;
    mountPath: string;
    subPath?: string;
    readOnly?: boolean;
  }>;
  livenessProbe?: K8sProbe;
  readinessProbe?: K8sProbe;
  startupProbe?: K8sProbe;
  lifecycle?: {
    postStart?: K8sHandler;
    preStop?: K8sHandler;
  };
  terminationMessagePath?: string;
  terminationMessagePolicy?: 'File' | 'FallbackToLogsOnError';
  imagePullPolicy?: 'Always' | 'Never' | 'IfNotPresent';
  securityContext?: K8sSecurityContext;
  stdin?: boolean;
  stdinOnce?: boolean;
  tty?: boolean;
}

export interface K8sProbe {
  exec?: {
    command: string[];
  };
  httpGet?: {
    path?: string;
    port: number | string;
    host?: string;
    scheme?: 'HTTP' | 'HTTPS';
    httpHeaders?: Array<{
      name: string;
      value: string;
    }>;
  };
  tcpSocket?: {
    port: number | string;
    host?: string;
  };
  initialDelaySeconds?: number;
  timeoutSeconds?: number;
  periodSeconds?: number;
  successThreshold?: number;
  failureThreshold?: number;
}

export interface K8sHandler {
  exec?: {
    command: string[];
  };
  httpGet?: {
    path?: string;
    port: number | string;
    host?: string;
    scheme?: 'HTTP' | 'HTTPS';
    httpHeaders?: Array<{
      name: string;
      value: string;
    }>;
  };
  tcpSocket?: {
    port: number | string;
    host?: string;
  };
}

export interface K8sVolume {
  name: string;
  hostPath?: {
    path: string;
    type?: string;
  };
  emptyDir?: {
    medium?: string;
    sizeLimit?: string;
  };
  configMap?: {
    name: string;
    items?: Array<{
      key: string;
      path: string;
      mode?: number;
    }>;
    defaultMode?: number;
    optional?: boolean;
  };
  secret?: {
    secretName: string;
    items?: Array<{
      key: string;
      path: string;
      mode?: number;
    }>;
    defaultMode?: number;
    optional?: boolean;
  };
  persistentVolumeClaim?: {
    claimName: string;
    readOnly?: boolean;
  };
}

export interface K8sCondition {
  type: string;
  status: 'True' | 'False' | 'Unknown';
  lastUpdateTime?: string;
  lastTransitionTime?: string;
  reason?: string;
  message?: string;
}

export interface K8sPodSecurityContext {
  seLinuxOptions?: {
    level?: string;
    role?: string;
    type?: string;
    user?: string;
  };
  windowsOptions?: {
    gmsaCredentialSpecName?: string;
    gmsaCredentialSpec?: string;
    runAsUserName?: string;
  };
  runAsUser?: number;
  runAsGroup?: number;
  runAsNonRoot?: boolean;
  fsGroup?: number;
  fsGroupChangePolicy?: 'Always' | 'OnRootMismatch';
  seccompProfile?: {
    type: 'RuntimeDefault' | 'Unconfined' | 'Localhost';
    localhostProfile?: string;
  };
  supplementalGroups?: number[];
  sysctls?: Array<{
    name: string;
    value: string;
  }>;
}

export interface K8sSecurityContext {
  capabilities?: {
    add?: string[];
    drop?: string[];
  };
  privileged?: boolean;
  seLinuxOptions?: {
    level?: string;
    role?: string;
    type?: string;
    user?: string;
  };
  windowsOptions?: {
    gmsaCredentialSpecName?: string;
    gmsaCredentialSpec?: string;
    runAsUserName?: string;
  };
  runAsUser?: number;
  runAsGroup?: number;
  runAsNonRoot?: boolean;
  readOnlyRootFilesystem?: boolean;
  allowPrivilegeEscalation?: boolean;
  procMount?: 'Default' | 'Unmasked';
  seccompProfile?: {
    type: 'RuntimeDefault' | 'Unconfined' | 'Localhost';
    localhostProfile?: string;
  };
}

export interface K8sAffinity {
  nodeAffinity?: {
    requiredDuringSchedulingIgnoredDuringExecution?: {
      nodeSelectorTerms: Array<{
        matchExpressions?: Array<{
          key: string;
          operator: 'In' | 'NotIn' | 'Exists' | 'DoesNotExist' | 'Gt' | 'Lt';
          values?: string[];
        }>;
        matchFields?: Array<{
          key: string;
          operator: 'In' | 'NotIn' | 'Exists' | 'DoesNotExist' | 'Gt' | 'Lt';
          values?: string[];
        }>;
      }>;
    };
    preferredDuringSchedulingIgnoredDuringExecution?: Array<{
      weight: number;
      preference: {
        matchExpressions?: Array<{
          key: string;
          operator: 'In' | 'NotIn' | 'Exists' | 'DoesNotExist' | 'Gt' | 'Lt';
          values?: string[];
        }>;
        matchFields?: Array<{
          key: string;
          operator: 'In' | 'NotIn' | 'Exists' | 'DoesNotExist' | 'Gt' | 'Lt';
          values?: string[];
        }>;
      };
    }>;
  };
  podAffinity?: K8sPodAffinity;
  podAntiAffinity?: K8sPodAffinity;
}

export interface K8sPodAffinity {
  requiredDuringSchedulingIgnoredDuringExecution?: Array<{
    labelSelector?: {
      matchLabels?: Record<string, string>;
      matchExpressions?: Array<{
        key: string;
        operator: 'In' | 'NotIn' | 'Exists' | 'DoesNotExist';
        values?: string[];
      }>;
    };
    namespaces?: string[];
    topologyKey: string;
  }>;
  preferredDuringSchedulingIgnoredDuringExecution?: Array<{
    weight: number;
    podAffinityTerm: {
      labelSelector?: {
        matchLabels?: Record<string, string>;
        matchExpressions?: Array<{
          key: string;
          operator: 'In' | 'NotIn' | 'Exists' | 'DoesNotExist';
          values?: string[];
        }>;
      };
      namespaces?: string[];
      topologyKey: string;
    };
  }>;
}

export interface K8sToleration {
  key?: string;
  operator?: 'Exists' | 'Equal';
  value?: string;
  effect?: 'NoSchedule' | 'PreferNoSchedule' | 'NoExecute';
  tolerationSeconds?: number;
}

export interface K8sPodDNSConfig {
  nameservers?: string[];
  searches?: string[];
  options?: Array<{
    name: string;
    value?: string;
  }>;
}

// API response types
export interface DeploymentResult {
  ready: boolean;
  replicas: {
    desired: number;
    current: number;
    ready: number;
    available: number;
  };
  conditions: K8sCondition[];
}

export interface K8sResourceStatus {
  kind: string;
  name: string;
  namespace: string;
  status: 'Ready' | 'NotReady' | 'Unknown';
  message?: string;
}

// Validation and configuration
export interface ManifestValidationResult {
  valid: boolean;
  errors: Array<{
    path: string;
    message: string;
    severity: 'error' | 'warning';
  }>;
  warnings: Array<{
    path: string;
    message: string;
  }>;
}

export interface ClusterInfo {
  version: string;
  platform: string;
  nodeCount: number;
  namespaces: string[];
  storageClasses: string[];
  ingressClasses: string[];
}

// Self subject access review for permission checking
export interface SelfSubjectAccessReview {
  apiVersion: 'authorization.k8s.io/v1';
  kind: 'SelfSubjectAccessReview';
  spec: {
    resourceAttributes?: {
      namespace?: string;
      verb: string;
      group?: string;
      version?: string;
      resource?: string;
      subresource?: string;
      name?: string;
    };
    nonResourceAttributes?: {
      path: string;
      verb: string;
    };
  };
  status?: {
    allowed: boolean;
    denied?: boolean;
    reason?: string;
    evaluationError?: string;
  };
}

// Generic K8s API error response
export interface K8sAPIError {
  kind: 'Status';
  apiVersion: 'v1';
  metadata: Record<string, unknown>;
  status: 'Failure';
  message: string;
  reason: string;
  code: number;
  details?: {
    name?: string;
    group?: string;
    kind?: string;
    uid?: string;
    causes?: Array<{
      reason: string;
      message: string;
      field: string;
    }>;
    retryAfterSeconds?: number;
  };
}

// Manifest collection types
export type K8sResource =
  | K8sDeployment
  | K8sService
  | K8sIngress
  | K8sConfigMap
  | K8sSecret
  | K8sPersistentVolumeClaim
  | K8sManifest;

export interface K8sManifestCollection {
  manifests: K8sResource[];
  namespace?: string;
  metadata?: {
    applicationName?: string;
    version?: string;
    description?: string;
  };
}

// K8s API client types for flexible resource handling
export interface K8sAPIClient {
  // Common methods across all API clients
  create?: (body: K8sAPIObject) => Promise<{ body: K8sAPIObject }>;
  read?: (name: string, namespace?: string) => Promise<{ body: K8sAPIObject }>;
  patch?: (
    name: string,
    body: Partial<K8sAPIObject>,
    namespace?: string,
  ) => Promise<{ body: K8sAPIObject }>;
  delete?: (name: string, namespace?: string) => Promise<{ body: K8sAPIObject }>;
  list?: (namespace?: string) => Promise<{ body: { items: K8sAPIObject[] } }>;

  // Namespaced resource methods
  createNamespacedCustomObject?: (
    group: string,
    version: string,
    namespace: string,
    plural: string,
    body: K8sAPIObject,
  ) => Promise<{ body: K8sAPIObject }>;
  patchNamespacedCustomObject?: (
    group: string,
    version: string,
    namespace: string,
    plural: string,
    name: string,
    body: Partial<K8sAPIObject>,
  ) => Promise<{ body: K8sAPIObject }>;

  // Core API methods
  createNamespacedPod?: (namespace: string, body: K8sAPIObject) => Promise<{ body: K8sAPIObject }>;
  createNamespacedService?: (
    namespace: string,
    body: K8sAPIObject,
  ) => Promise<{ body: K8sAPIObject }>;
  createNamespacedConfigMap?: (
    namespace: string,
    body: K8sAPIObject,
  ) => Promise<{ body: K8sAPIObject }>;
  createNamespacedSecret?: (
    namespace: string,
    body: K8sAPIObject,
  ) => Promise<{ body: K8sAPIObject }>;

  // Apps API methods
  createNamespacedDeployment?: (
    namespace: string,
    body: K8sAPIObject,
  ) => Promise<{ body: K8sAPIObject }>;
  createNamespacedStatefulSet?: (
    namespace: string,
    body: K8sAPIObject,
  ) => Promise<{ body: K8sAPIObject }>;
  createNamespacedDaemonSet?: (
    namespace: string,
    body: K8sAPIObject,
  ) => Promise<{ body: K8sAPIObject }>;

  // Networking API methods
  createNamespacedIngress?: (
    namespace: string,
    body: K8sAPIObject,
  ) => Promise<{ body: K8sAPIObject }>;

  // Patch methods
  patchNamespacedPod?: (
    name: string,
    namespace: string,
    body: Partial<K8sAPIObject>,
  ) => Promise<{ body: K8sAPIObject }>;
  patchNamespacedService?: (
    name: string,
    namespace: string,
    body: Partial<K8sAPIObject>,
  ) => Promise<{ body: K8sAPIObject }>;
  patchNamespacedDeployment?: (
    name: string,
    namespace: string,
    body: Partial<K8sAPIObject>,
  ) => Promise<{ body: K8sAPIObject }>;
  patchNamespacedConfigMap?: (
    name: string,
    namespace: string,
    body: Partial<K8sAPIObject>,
  ) => Promise<{ body: K8sAPIObject }>;
  patchNamespacedSecret?: (
    name: string,
    namespace: string,
    body: Partial<K8sAPIObject>,
  ) => Promise<{ body: K8sAPIObject }>;
  patchNamespacedStatefulSet?: (
    name: string,
    namespace: string,
    body: Partial<K8sAPIObject>,
  ) => Promise<{ body: K8sAPIObject }>;
  patchNamespacedDaemonSet?: (
    name: string,
    namespace: string,
    body: Partial<K8sAPIObject>,
  ) => Promise<{ body: K8sAPIObject }>;
  patchNamespacedIngress?: (
    name: string,
    namespace: string,
    body: Partial<K8sAPIObject>,
  ) => Promise<{ body: K8sAPIObject }>;
}

// Error type for K8s API operations
export interface K8sAPIOperationError extends Error {
  statusCode?: number;
  response?: {
    statusCode?: number;
    body?: K8sAPIError;
  };
}
