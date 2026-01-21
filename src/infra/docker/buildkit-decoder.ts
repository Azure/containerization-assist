/**
 * BuildKit trace decoder for moby.buildkit.trace events
 * Decodes protobuf-encoded BuildKit status messages into human-readable logs
 */

import protobuf from 'protobufjs';
import type { Logger } from 'pino';

// Lazily loaded protobuf root
let protoRoot: protobuf.Root | null = null;

/**
 * Initialize the protobuf schema
 * Define the schema inline to avoid file I/O
 */
function initializeProto(logger: Logger): protobuf.Root {
  if (protoRoot) {
    return protoRoot;
  }

  try {
    // Define the protobuf schema programmatically
    protoRoot = new protobuf.Root();

    // Define the StatusResponse message and its nested types
    const StatusResponse = new protobuf.Type('StatusResponse')
      .add(new protobuf.Field('vertexes', 1, 'Vertex', 'repeated'))
      .add(new protobuf.Field('statuses', 2, 'VertexStatus', 'repeated'))
      .add(new protobuf.Field('logs', 3, 'VertexLog', 'repeated'))
      .add(new protobuf.Field('warnings', 4, 'VertexWarning', 'repeated'));

    const Vertex = new protobuf.Type('Vertex')
      .add(new protobuf.Field('digest', 1, 'string'))
      .add(new protobuf.Field('inputs', 2, 'string', 'repeated'))
      .add(new protobuf.Field('name', 3, 'string'))
      .add(new protobuf.Field('cached', 4, 'bool'))
      .add(new protobuf.Field('started', 5, 'google.protobuf.Timestamp'))
      .add(new protobuf.Field('completed', 6, 'google.protobuf.Timestamp'))
      .add(new protobuf.Field('error', 7, 'string'));

    const VertexStatus = new protobuf.Type('VertexStatus')
      .add(new protobuf.Field('ID', 1, 'string'))
      .add(new protobuf.Field('vertex', 2, 'string'))
      .add(new protobuf.Field('name', 3, 'string'))
      .add(new protobuf.Field('current', 4, 'int64'))
      .add(new protobuf.Field('total', 5, 'int64'))
      .add(new protobuf.Field('timestamp', 6, 'google.protobuf.Timestamp'))
      .add(new protobuf.Field('started', 7, 'google.protobuf.Timestamp'))
      .add(new protobuf.Field('completed', 8, 'google.protobuf.Timestamp'));

    const VertexLog = new protobuf.Type('VertexLog')
      .add(new protobuf.Field('vertex', 1, 'string'))
      .add(new protobuf.Field('timestamp', 2, 'google.protobuf.Timestamp'))
      .add(new protobuf.Field('stream', 3, 'int64'))
      .add(new protobuf.Field('msg', 4, 'bytes'));

    const VertexWarning = new protobuf.Type('VertexWarning')
      .add(new protobuf.Field('vertex', 1, 'string'))
      .add(new protobuf.Field('level', 2, 'int64'))
      .add(new protobuf.Field('short', 3, 'bytes'))
      .add(new protobuf.Field('detail', 4, 'bytes', 'repeated'))
      .add(new protobuf.Field('url', 5, 'string'));

    // Google Timestamp type
    const Timestamp = new protobuf.Type('Timestamp')
      .add(new protobuf.Field('seconds', 1, 'int64'))
      .add(new protobuf.Field('nanos', 2, 'int32'));

    // Add to namespace
    const googleProtobuf = new protobuf.Namespace('google.protobuf');
    googleProtobuf.add(Timestamp);

    const mobyBuildkit = new protobuf.Namespace('moby.buildkit.v1');
    mobyBuildkit.add(StatusResponse);
    mobyBuildkit.add(Vertex);
    mobyBuildkit.add(VertexStatus);
    mobyBuildkit.add(VertexLog);
    mobyBuildkit.add(VertexWarning);

    protoRoot.add(googleProtobuf);
    protoRoot.add(mobyBuildkit);

    return protoRoot;
  } catch (error) {
    logger.error(
      { error, errorMessage: error instanceof Error ? error.message : String(error) },
      'Failed to initialize BuildKit protobuf schema',
    );
    throw error;
  }
}

/**
 * Decoded BuildKit status response
 */
export interface BuildKitStatus {
  /** Completed build steps */
  steps: string[];
  /** Log messages */
  logs: string[];
  /** Warnings */
  warnings: string[];
  /** Errors */
  errors: string[];
}

/**
 * Decode a BuildKit trace event from base64-encoded protobuf
 *
 * @param auxData - Base64-encoded protobuf data from moby.buildkit.trace event
 * @param logger - Logger for debugging
 * @returns Decoded status with human-readable messages, or null if decoding fails
 */
export function decodeBuildKitTrace(auxData: unknown, logger: Logger): BuildKitStatus | null {
  if (!auxData || typeof auxData !== 'string') {
    return null;
  }

  try {
    // Initialize protobuf schema
    const root = initializeProto(logger);
    const StatusResponse = root.lookupType('moby.buildkit.v1.StatusResponse');

    // Decode base64 → bytes → protobuf
    const bytes = Buffer.from(auxData, 'base64');

    const message = StatusResponse.decode(bytes);
    const obj = StatusResponse.toObject(message, {
      longs: Number,
      bytes: Buffer, // Return bytes as Buffer instead of base64 string
      defaults: false,
      arrays: true,
      objects: true,
      oneofs: true,
    });

    const status: BuildKitStatus = {
      steps: [],
      logs: [],
      warnings: [],
      errors: [],
    };

    // Extract completed vertices (build steps)
    if (obj.vertexes && Array.isArray(obj.vertexes)) {
      for (const vertex of obj.vertexes) {
        if (vertex.started && vertex.completed && vertex.name) {
          status.steps.push(vertex.name);
        }
        if (vertex.error) {
          status.errors.push(`${vertex.name}: ${vertex.error}`);
        }
      }
    }

    // Extract vertex logs
    if (obj.logs && Array.isArray(obj.logs)) {
      for (const log of obj.logs) {
        if (log.msg) {
          // msg is a Buffer, convert to string
          const logText = Buffer.isBuffer(log.msg) ? log.msg.toString('utf-8') : String(log.msg);
          const trimmed = logText.trim();
          if (trimmed) {
            status.logs.push(trimmed);
          }
        }
      }
    }

    // Extract warnings
    if (obj.warnings && Array.isArray(obj.warnings)) {
      for (const warning of obj.warnings) {
        if (warning.short) {
          const warnText = Buffer.isBuffer(warning.short)
            ? warning.short.toString('utf-8')
            : String(warning.short);
          status.warnings.push(warnText.trim());
        }
      }
    }

    return status;
  } catch (error) {
    logger.debug({ error }, 'Failed to decode BuildKit protobuf');
    return null;
  }
}

/**
 * Format BuildKit status into a single log message
 * Returns the most relevant message (step, log, or warning)
 */
export function formatBuildKitStatus(status: BuildKitStatus): string | null {
  // Prioritize errors
  if (status.errors.length > 0) {
    return status.errors[status.errors.length - 1] ?? null;
  }

  // Then completed steps
  if (status.steps.length > 0) {
    return status.steps[status.steps.length - 1] ?? null;
  }

  // Then logs
  if (status.logs.length > 0) {
    return status.logs[status.logs.length - 1] ?? null;
  }

  // Finally warnings
  if (status.warnings.length > 0) {
    const warning = status.warnings[status.warnings.length - 1];
    return warning ? `⚠️  ${warning}` : null;
  }

  return null;
}
