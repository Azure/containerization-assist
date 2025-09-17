/**
 * Generate Helm Charts Tool
 *
 * Generates Helm chart structure with templates and values
 * Following the same simplified patterns as other tools
 */

import path from 'path';
import { getToolLogger, createToolTimer } from '@lib/tool-helpers';
import { extractErrorMessage } from '@lib/error-utils';
import { promises as fs } from 'node:fs';
import { ensureSession, defineToolIO, useSessionSlice } from '@mcp/tool-session-helpers';
// AI imports commented out for now - can be added later for enhanced generation
// import { aiGenerateWithSampling } from '@mcp/tool-ai-helpers';
// import { enhancePromptWithKnowledge } from '@lib/ai-knowledge-enhancer';
// import type { SamplingOptions } from '@lib/sampling';
import { createStandardProgress } from '@mcp/progress-helper';
import type { ToolContext } from '@mcp/context';
import type { SessionData } from '@tools/session-types';
import { Success, Failure, type Result } from '@types';
// import { stripFencesAndNoise } from '@lib/text-processing';
import { execSync } from 'child_process';
import * as yaml from 'js-yaml';
import { generateHelmChartsSchema, type GenerateHelmChartsParams } from './schema';
import { z } from 'zod';

// Define the result schema for type safety
const GenerateHelmChartsResultSchema = z.object({
  chartPath: z.string(),
  chartName: z.string(),
  files: z.array(z.string()),
  validationResult: z
    .object({
      passed: z.boolean(),
      output: z.string(),
      warnings: z.array(z.string()).optional(),
      errors: z.array(z.string()).optional(),
    })
    .optional(),
  warnings: z.array(z.string()).optional(),
  sessionId: z.string().optional(),
  samplingMetadata: z
    .object({
      stoppedEarly: z.boolean().optional(),
      candidatesGenerated: z.number(),
      winnerScore: z.number(),
      samplingDuration: z.number().optional(),
    })
    .optional(),
});

// Define tool IO for type-safe session operations
const io = defineToolIO(generateHelmChartsSchema, GenerateHelmChartsResultSchema);

// Tool-specific state schema
const StateSchema = z.object({
  lastGeneratedAt: z.date().optional(),
  chartCount: z.number().optional(),
  lastChartName: z.string().optional(),
  lastChartVersion: z.string().optional(),
  validationPassed: z.boolean().optional(),
});

/**
 * Result from Helm chart generation
 */
export interface GenerateHelmChartsResult {
  /** Generated chart path */
  chartPath: string;
  /** Chart name */
  chartName: string;
  /** List of generated files */
  files: string[];
  /** Validation results if run */
  validationResult?: {
    passed: boolean;
    output: string;
    warnings?: string[];
    errors?: string[];
  };
  /** Warnings about chart configuration */
  warnings?: string[];
  /** Session ID for reference */
  sessionId?: string;
  /** Sampling metadata if used */
  samplingMetadata?: {
    stoppedEarly?: boolean;
    candidatesGenerated: number;
    winnerScore: number;
    samplingDuration?: number;
  };
}

/**
 * Helm chart structure
 */
interface HelmChart {
  chartYaml: {
    apiVersion: string;
    name: string;
    description: string;
    type: string;
    version: string;
    appVersion: string;
  };
  valuesYaml: Record<string, any>;
  templates: Record<string, string>;
}

/**
 * Generate basic Helm chart structure (fallback)
 */
function generateBasicHelmChart(params: GenerateHelmChartsParams): HelmChart {
  const {
    chartName,
    appName,
    imageId,
    chartVersion = '0.1.0',
    appVersion = '1.0.0',
    description = `A Helm chart for ${appName}`,
    replicas = 1,
    port = 8080,
    serviceType = 'ClusterIP',
    ingressEnabled = false,
    ingressHost,
    ingressClass = 'nginx',
    resources,
    healthCheck,
    autoscaling,
  } = params;

  // Chart.yaml
  const chartYaml = {
    apiVersion: 'v2',
    name: chartName,
    description,
    type: 'application',
    version: chartVersion,
    appVersion,
  };

  // values.yaml - following Helm best practices
  const valuesYaml = {
    replicaCount: replicas,

    image: {
      repository: imageId.split(':')[0] || imageId,
      pullPolicy: 'IfNotPresent',
      tag: imageId.split(':')[1] || 'latest',
    },

    imagePullSecrets: [],
    nameOverride: '',
    fullnameOverride: '',

    serviceAccount: {
      create: true,
      automount: true,
      annotations: {},
      name: '',
    },

    podAnnotations: {},
    podLabels: {},

    podSecurityContext: {},
    securityContext: {},

    service: {
      type: serviceType,
      port,
    },

    ingress: {
      enabled: ingressEnabled,
      className: ingressClass,
      annotations: {},
      hosts: ingressHost
        ? [
            {
              host: ingressHost,
              paths: [
                {
                  path: '/',
                  pathType: 'ImplementationSpecific',
                },
              ],
            },
          ]
        : [],
      tls: [],
    },

    resources: resources || {
      limits: {
        cpu: '200m',
        memory: '256Mi',
      },
      requests: {
        cpu: '100m',
        memory: '128Mi',
      },
    },

    livenessProbe: healthCheck?.enabled
      ? {
          httpGet: {
            path: healthCheck.path || '/health',
            port: 'http',
          },
          initialDelaySeconds: healthCheck.initialDelaySeconds || 30,
        }
      : {},

    readinessProbe: healthCheck?.enabled
      ? {
          httpGet: {
            path: healthCheck.path || '/health',
            port: 'http',
          },
          initialDelaySeconds: 5,
        }
      : {},

    autoscaling: {
      enabled: autoscaling?.enabled || false,
      minReplicas: autoscaling?.minReplicas || 1,
      maxReplicas: autoscaling?.maxReplicas || 10,
      targetCPUUtilizationPercentage: autoscaling?.targetCPUUtilizationPercentage || 70,
    },

    volumes: [],
    volumeMounts: [],

    nodeSelector: {},
    tolerations: [],
    affinity: {},
  };

  // Templates
  const templates: Record<string, string> = {};

  // _helpers.tpl
  templates['_helpers.tpl'] = `{{/*
Expand the name of the chart.
*/}}
{{- define "${chartName}.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "${chartName}.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "${chartName}.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "${chartName}.labels" -}}
helm.sh/chart: {{ include "${chartName}.chart" . }}
{{ include "${chartName}.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "${chartName}.selectorLabels" -}}
app.kubernetes.io/name: {{ include "${chartName}.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "${chartName}.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "${chartName}.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}`;

  // deployment.yaml
  templates['deployment.yaml'] = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "${chartName}.fullname" . }}
  labels:
    {{- include "${chartName}.labels" . | nindent 4 }}
spec:
  {{- if not .Values.autoscaling.enabled }}
  replicas: {{ .Values.replicaCount }}
  {{- end }}
  selector:
    matchLabels:
      {{- include "${chartName}.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      {{- with .Values.podAnnotations }}
      annotations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      labels:
        {{- include "${chartName}.labels" . | nindent 8 }}
        {{- with .Values.podLabels }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
    spec:
      {{- with .Values.imagePullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      serviceAccountName: {{ include "${chartName}.serviceAccountName" . }}
      securityContext:
        {{- toYaml .Values.podSecurityContext | nindent 8 }}
      containers:
        - name: {{ .Chart.Name }}
          securityContext:
            {{- toYaml .Values.securityContext | nindent 12 }}
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          ports:
            - name: http
              containerPort: {{ .Values.service.port }}
              protocol: TCP
          {{- with .Values.livenessProbe }}
          livenessProbe:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          {{- with .Values.readinessProbe }}
          readinessProbe:
            {{- toYaml . | nindent 12 }}
          {{- end }}
          resources:
            {{- toYaml .Values.resources | nindent 12 }}
          {{- with .Values.volumeMounts }}
          volumeMounts:
            {{- toYaml . | nindent 12 }}
          {{- end }}
      {{- with .Values.volumes }}
      volumes:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.affinity }}
      affinity:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.tolerations }}
      tolerations:
        {{- toYaml . | nindent 8 }}
      {{- end }}`;

  // service.yaml
  templates['service.yaml'] = `apiVersion: v1
kind: Service
metadata:
  name: {{ include "${chartName}.fullname" . }}
  labels:
    {{- include "${chartName}.labels" . | nindent 4 }}
spec:
  type: {{ .Values.service.type }}
  ports:
    - port: {{ .Values.service.port }}
      targetPort: http
      protocol: TCP
      name: http
  selector:
    {{- include "${chartName}.selectorLabels" . | nindent 4 }}`;

  // serviceaccount.yaml
  templates['serviceaccount.yaml'] = `{{- if .Values.serviceAccount.create -}}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "${chartName}.serviceAccountName" . }}
  labels:
    {{- include "${chartName}.labels" . | nindent 4 }}
  {{- with .Values.serviceAccount.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
automountServiceAccountToken: {{ .Values.serviceAccount.automount }}
{{- end }}`;

  // ingress.yaml
  templates['ingress.yaml'] = `{{- if .Values.ingress.enabled -}}
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ include "${chartName}.fullname" . }}
  labels:
    {{- include "${chartName}.labels" . | nindent 4 }}
  {{- with .Values.ingress.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  {{- if .Values.ingress.className }}
  ingressClassName: {{ .Values.ingress.className }}
  {{- end }}
  {{- if .Values.ingress.tls }}
  tls:
    {{- range .Values.ingress.tls }}
    - hosts:
        {{- range .hosts }}
        - {{ . | quote }}
        {{- end }}
      secretName: {{ .secretName }}
    {{- end }}
  {{- end }}
  rules:
    {{- range .Values.ingress.hosts }}
    - host: {{ .host | quote }}
      http:
        paths:
          {{- range .paths }}
          - path: {{ .path }}
            {{- if .pathType }}
            pathType: {{ .pathType }}
            {{- end }}
            backend:
              service:
                name: {{ include "${chartName}.fullname" $ }}
                port:
                  number: {{ $.Values.service.port }}
          {{- end }}
    {{- end }}
{{- end }}`;

  // hpa.yaml
  templates['hpa.yaml'] = `{{- if .Values.autoscaling.enabled }}
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: {{ include "${chartName}.fullname" . }}
  labels:
    {{- include "${chartName}.labels" . | nindent 4 }}
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: {{ include "${chartName}.fullname" . }}
  minReplicas: {{ .Values.autoscaling.minReplicas }}
  maxReplicas: {{ .Values.autoscaling.maxReplicas }}
  metrics:
    {{- if .Values.autoscaling.targetCPUUtilizationPercentage }}
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: {{ .Values.autoscaling.targetCPUUtilizationPercentage }}
    {{- end }}
    {{- if .Values.autoscaling.targetMemoryUtilizationPercentage }}
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: {{ .Values.autoscaling.targetMemoryUtilizationPercentage }}
    {{- end }}
{{- end }}`;

  // NOTES.txt
  templates['NOTES.txt'] = `1. Get the application URL by running these commands:
{{- if .Values.ingress.enabled }}
{{- range $host := .Values.ingress.hosts }}
  {{- range .paths }}
  http{{ if $.Values.ingress.tls }}s{{ end }}://{{ $host.host }}{{ .path }}
  {{- end }}
{{- end }}
{{- else if contains "NodePort" .Values.service.type }}
  export NODE_PORT=$(kubectl get --namespace {{ .Release.Namespace }} -o jsonpath="{.spec.ports[0].nodePort}" services {{ include "${chartName}.fullname" . }})
  export NODE_IP=$(kubectl get nodes --namespace {{ .Release.Namespace }} -o jsonpath="{.items[0].status.addresses[0].address}")
  echo http://$NODE_IP:$NODE_PORT
{{- else if contains "LoadBalancer" .Values.service.type }}
     NOTE: It may take a few minutes for the LoadBalancer IP to be available.
           You can watch the status of by running 'kubectl get --namespace {{ .Release.Namespace }} svc -w {{ include "${chartName}.fullname" . }}'
  export SERVICE_IP=$(kubectl get svc --namespace {{ .Release.Namespace }} {{ include "${chartName}.fullname" . }} --template "{{ "{{" }} range (index .status.loadBalancer.ingress 0) {{ "}}" }}{{ "{{" }}.{{ "}}" }}{{ "{{" }} end {{ "}}" }}")
  echo http://$SERVICE_IP:{{ .Values.service.port }}
{{- else if contains "ClusterIP" .Values.service.type }}
  export POD_NAME=$(kubectl get pods --namespace {{ .Release.Namespace }} -l "app.kubernetes.io/name={{ include "${chartName}.name" . }},app.kubernetes.io/instance={{ .Release.Name }}" -o jsonpath="{.items[0].metadata.name}")
  export CONTAINER_PORT=$(kubectl get pod --namespace {{ .Release.Namespace }} $POD_NAME -o jsonpath="{.spec.containers[0].ports[0].containerPort}")
  echo "Visit http://127.0.0.1:8080 to use your application"
  kubectl --namespace {{ .Release.Namespace }} port-forward $POD_NAME 8080:$CONTAINER_PORT
{{- end }}`;

  return {
    chartYaml,
    valuesYaml,
    templates,
  };
}

/**
 * Run Helm lint validation
 */
async function validateHelmChart(
  chartPath: string,
  strict: boolean,
  logger: any,
): Promise<{ passed: boolean; output: string; warnings?: string[]; errors?: string[] }> {
  try {
    // Check if helm is installed
    try {
      execSync('helm version --short', { stdio: 'pipe' });
    } catch {
      logger.warn('Helm CLI not found, skipping validation');
      return {
        passed: true,
        output: 'Helm CLI not found, validation skipped',
        warnings: ['Helm CLI not installed - install it for chart validation'],
      };
    }

    // Run helm lint
    const strictFlag = strict ? '--strict' : '';
    const result = execSync(`helm lint ${strictFlag} ${chartPath}`, {
      encoding: 'utf-8',
      stdio: 'pipe',
    });

    return {
      passed: true,
      output: result,
    };
  } catch (error: any) {
    const output = error.stdout || error.message;
    const warnings: string[] = [];
    const errors: string[] = [];

    // Parse helm lint output for warnings and errors
    const lines = output.split('\n');
    for (const line of lines) {
      if (line.includes('[WARNING]')) {
        warnings.push(line);
      } else if (line.includes('[ERROR]')) {
        errors.push(line);
      }
    }

    return {
      passed: errors.length === 0 && (!strict || warnings.length === 0),
      output,
      warnings,
      errors,
    };
  }
}

/**
 * Generate Helm charts implementation
 */
async function generateHelmChartsImpl(
  params: GenerateHelmChartsParams,
  context: ToolContext,
): Promise<Result<GenerateHelmChartsResult>> {
  // Basic parameter validation
  if (!params || typeof params !== 'object') {
    return Failure('Invalid parameters provided');
  }

  // Progress reporting
  const progress = context.progress ? createStandardProgress(context.progress) : undefined;
  const logger = getToolLogger(context, 'generate-helm-charts');
  const timer = createToolTimer(logger, 'generate-helm-charts');

  try {
    const { chartName, appName } = params;

    // Progress: Starting validation
    if (progress) await progress('VALIDATING');

    // Ensure session exists and get typed slice operations
    const sessionResult = await ensureSession(context, params.sessionId);
    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }

    const { id: sessionId, state: session } = sessionResult.value;
    const slice = useSessionSlice('generate-helm-charts', io, context, StateSchema);

    if (!slice) {
      return Failure('Session manager not available');
    }

    // Record input in session slice
    await slice.patch(sessionId, { input: params });

    const sessionData = session as unknown as SessionData;

    // Get image from session or params
    const buildResult = (sessionData?.results?.['build-image'] ||
      sessionData?.workflowState?.results?.['build-image']) as
      | { tags?: string[]; imageId?: string }
      | undefined;
    const imageId = params.imageId || buildResult?.tags?.[0] || `${appName}:latest`;

    // Progress: Executing generation
    if (progress) await progress('EXECUTING');

    // Generate Helm chart structure (simplified - no AI for now, just templates)
    const chart = generateBasicHelmChart({ ...params, imageId });

    // Progress: Writing files
    if (progress) await progress('FINALIZING');

    // Write chart to disk - use current directory as base
    const chartPath = path.join('.', 'helm', chartName);
    const templatesPath = path.join(chartPath, 'templates');

    // Create directories
    await fs.mkdir(templatesPath, { recursive: true });

    // Write Chart.yaml
    await fs.writeFile(
      path.join(chartPath, 'Chart.yaml'),
      yaml.dump(chart.chartYaml, { noRefs: true }),
      'utf-8',
    );

    // Write values.yaml
    await fs.writeFile(
      path.join(chartPath, 'values.yaml'),
      yaml.dump(chart.valuesYaml, { noRefs: true, lineWidth: -1 }),
      'utf-8',
    );

    // Write templates
    const files = ['Chart.yaml', 'values.yaml'];
    for (const [filename, content] of Object.entries(chart.templates)) {
      const filePath = path.join(templatesPath, filename);
      await fs.writeFile(filePath, content, 'utf-8');
      files.push(`templates/${filename}`);
    }

    // Write .helmignore
    const helmignore = `# Patterns to ignore when building packages.
# This supports shell glob matching, relative path matching, and
# negation (prefixed with !). Only one pattern per line.
.DS_Store
# Common VCS dirs
.git/
.gitignore
.bzr/
.bzrignore
.hg/
.hgignore
.svn/
# Common backup files
*.swp
*.bak
*.tmp
*.orig
*~
# Various IDEs
.project
.idea/
*.tmproj
.vscode/`;
    await fs.writeFile(path.join(chartPath, '.helmignore'), helmignore, 'utf-8');
    files.push('.helmignore');

    // Run validation if requested
    let validationResult;
    if (params.runValidation !== false) {
      validationResult = await validateHelmChart(
        chartPath,
        params.strictValidation || false,
        logger,
      );

      if (!validationResult.passed && params.strictValidation) {
        logger.warn({ validationResult }, 'Helm chart validation failed');
      }
    }

    // Check for warnings
    const warnings: string[] = [];
    if (!params.resources) {
      warnings.push('No resource limits specified - consider adding for production');
    }
    if (!params.healthCheck?.enabled) {
      warnings.push('No health checks configured - consider adding for resilience');
    }
    if (params.replicas === 1 && params.environment === 'production') {
      warnings.push('Single replica in production - consider increasing for availability');
    }

    // Prepare result
    const result: GenerateHelmChartsResult = {
      chartPath,
      chartName,
      files,
      ...(validationResult && { validationResult }),
      ...(warnings.length > 0 && { warnings }),
      sessionId,
    };

    // Update typed session slice with output and state
    await slice.patch(sessionId, {
      output: result,
      state: {
        lastGeneratedAt: new Date(),
        chartCount: 1,
        lastChartName: chartName,
        lastChartVersion: chart.chartYaml.version,
        validationPassed: validationResult?.passed,
      },
    });

    // Progress: Complete
    if (progress) await progress('COMPLETE');
    timer.end({ chartPath });

    // Return result with file indicator
    const enrichedResult = {
      ...result,
      _fileWritten: true,
      _fileWrittenPath: chartPath,
    };

    return Success(enrichedResult);
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'Helm chart generation failed');
    return Failure(extractErrorMessage(error));
  }
}

/**
 * Generate Helm charts tool
 */
export const generateHelmCharts = generateHelmChartsImpl;
