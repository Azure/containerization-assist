const { getConnection, resetConnection } = require('./client');

// Session ID counter for more readable IDs
let sessionCounter = 0;

// Generate a unique session ID
function createSession() {
  const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, -5);
  const counter = (++sessionCounter).toString().padStart(4, '0');
  return `session-${timestamp}-${counter}`;
}

// Parse MCP response
function parseResult(mcpResult) {
  if (mcpResult?.content?.[0]?.text) {
    try {
      return JSON.parse(mcpResult.content[0].text);
    } catch (err) {
      // If it's not JSON, return as raw text
      return { raw: mcpResult.content[0].text };
    }
  }
  return mcpResult;
}

// Execute a tool with auto-session management and retry logic
async function executeTool(toolName, args = {}, retries = 1) {
  try {
    const connection = await getConnection();
    
    // Auto-generate session if not provided (except for utility tools)
    const utilityTools = ['list_tools', 'ping', 'server_status'];
    if (!args.session_id && !utilityTools.includes(toolName)) {
      args.session_id = createSession();
    }
    
    const result = await connection.callTool(toolName, args);
    return parseResult(result);
  } catch (error) {
    // Retry logic for connection failures
    if (retries > 0 && error.message.includes('Connection closed')) {
      resetConnection();
      return executeTool(toolName, args, retries - 1);
    }
    throw error;
  }
}

// ============================================
// Workflow Step Tools (10)
// ============================================

/**
 * Analyze a repository to detect language, framework, and dependencies
 * @param {string} repoPath - Path to the repository to analyze
 * @param {Object} options - Additional options (session_id, etc.)
 * @returns {Promise<Object>} Analysis results
 */
async function analyzeRepository(repoPath, options = {}) {
  if (!repoPath) {
    throw new Error('repoPath is required for analyzeRepository');
  }
  return executeTool('analyze_repository', {
    repo_path: repoPath,
    ...options
  });
}

/**
 * Generate a Dockerfile based on repository analysis
 * @param {Object} options - Generation options (session_id, base_image, etc.)
 * @returns {Promise<Object>} Dockerfile generation results
 */
async function generateDockerfile(options = {}) {
  return executeTool('generate_dockerfile', options);
}

/**
 * Build a Docker image from the generated Dockerfile
 * @param {Object} options - Build options (session_id, dockerfile, context, tags, etc.)
 * @returns {Promise<Object>} Build results with image details
 */
async function buildImage(options = {}) {
  return executeTool('build_image', options);
}

/**
 * Scan a Docker image for security vulnerabilities
 * @param {Object} options - Scan options (session_id, scanners, severity, etc.)
 * @returns {Promise<Object>} Scan results with vulnerability report
 */
async function scanImage(options = {}) {
  return executeTool('scan_image', options);
}

/**
 * Tag a Docker image with a version or label
 * @param {string} tag - The tag to apply to the image
 * @param {Object} options - Additional options (session_id, etc.)
 * @returns {Promise<Object>} Tagging results
 */
async function tagImage(tag, options = {}) {
  if (!tag) {
    throw new Error('tag is required for tagImage');
  }
  return executeTool('tag_image', {
    tag,
    ...options
  });
}

/**
 * Push a Docker image to a container registry
 * @param {string} registry - The registry URL to push to
 * @param {Object} options - Push options (session_id, credentials, etc.)
 * @returns {Promise<Object>} Push results with registry details
 */
async function pushImage(registry, options = {}) {
  if (!registry) {
    throw new Error('registry is required for pushImage');
  }
  return executeTool('push_image', {
    registry,
    ...options
  });
}

/**
 * Generate Kubernetes manifests for the application
 * @param {Object} options - Manifest options (session_id, namespace, replicas, etc.)
 * @returns {Promise<Object>} Generated manifests information
 */
async function generateK8sManifests(options = {}) {
  return executeTool('generate_k8s_manifests', options);
}

/**
 * Prepare the Kubernetes cluster for deployment
 * @param {Object} options - Cluster options (session_id, cluster_config, etc.)
 * @returns {Promise<Object>} Cluster preparation results
 */
async function prepareCluster(options = {}) {
  return executeTool('prepare_cluster', options);
}

/**
 * Deploy the application to Kubernetes
 * @param {Object} options - Deployment options (session_id, namespace, etc.)
 * @returns {Promise<Object>} Deployment results with status
 */
async function deployApplication(options = {}) {
  return executeTool('deploy_application', options);
}

/**
 * Verify the deployment is running correctly
 * @param {Object} options - Verification options (session_id, health_checks, etc.)
 * @returns {Promise<Object>} Verification results with endpoints
 */
async function verifyDeployment(options = {}) {
  return executeTool('verify_deployment', options);
}

// ============================================
// Utility Tools (3)
// ============================================

/**
 * List all available MCP tools
 * @returns {Promise<Object>} List of tools with descriptions
 */
async function listTools() {
  return executeTool('list_tools');
}

/**
 * Ping the MCP server to check connectivity
 * @returns {Promise<Object>} Ping response
 */
async function ping() {
  return executeTool('ping');
}

/**
 * Get the MCP server status
 * @returns {Promise<Object>} Server status information
 */
async function serverStatus() {
  return executeTool('server_status');
}

// ============================================
// Exports
// ============================================

module.exports = {
  // Workflow Step Tools (10)
  analyzeRepository,
  generateDockerfile,
  buildImage,
  scanImage,
  tagImage,
  pushImage,
  generateK8sManifests,
  prepareCluster,
  deployApplication,
  verifyDeployment,
  
  // Utility Tools (3)
  listTools,
  ping,
  serverStatus,
  
  // Session Management
  createSession,
  
  // Connection Management (advanced usage)
  disconnect: () => {
    const { resetConnection } = require('./client');
    resetConnection();
  }
};