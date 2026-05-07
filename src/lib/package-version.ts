import { GENERATED_PACKAGE_VERSION } from './generated-version';

export const PACKAGE_VERSION = GENERATED_PACKAGE_VERSION;

export const TOOL_NAME = 'containerization-assist';

// Dockerfile OCI labels
export const OCI_LABEL_VERSION = 'com.azure.containerizationassist.version';
export const OCI_LABEL_CREATED_BY = 'com.azure.containerizationassist.createdby';

// K8s labels & annotations
export const K8S_ANNOTATION_VERSION = 'com.azure.containerizationassist/version';
export const K8S_LABEL_MANAGED_BY = 'app.kubernetes.io/managed-by';
export const K8S_LABEL_NAME = 'app.kubernetes.io/name';
