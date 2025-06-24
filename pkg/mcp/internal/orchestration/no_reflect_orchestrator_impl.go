package orchestration

import (
	"context"
	"fmt"

	"github.com/Azure/container-copilot/pkg/mcp/internal/tools"
)

// Implementation of all tool execution methods for NoReflectToolOrchestrator

func (o *NoReflectToolOrchestrator) executeBuildImage(ctx context.Context, argsMap map[string]interface{}) (interface{}, error) {
	if o.toolFactory == nil {
		return nil, fmt.Errorf("tool factory not initialized")
	}

	// Create tool instance
	tool := o.toolFactory.CreateBuildImageTool()

	// Build typed arguments
	args := tools.AtomicBuildImageArgs{}

	// Extract required fields
	if sessionID, ok := getString(argsMap, "session_id"); ok {
		args.SessionID = sessionID
	} else {
		return nil, fmt.Errorf("session_id is required")
	}

	if imageName, ok := getString(argsMap, "image_name"); ok {
		args.ImageName = imageName
	} else {
		return nil, fmt.Errorf("image_name is required")
	}

	// Extract optional fields
	if imageTag, ok := getString(argsMap, "image_tag"); ok {
		args.ImageTag = imageTag
	}

	if dockerfilePath, ok := getString(argsMap, "dockerfile_path"); ok {
		args.DockerfilePath = dockerfilePath
	}

	if buildContext, ok := getString(argsMap, "build_context"); ok {
		args.BuildContext = buildContext
	}

	if platform, ok := getString(argsMap, "platform"); ok {
		args.Platform = platform
	}

	if noCache, ok := getBool(argsMap, "no_cache"); ok {
		args.NoCache = noCache
	}

	if buildArgs, ok := argsMap["build_args"].(map[string]interface{}); ok {
		args.BuildArgs = make(map[string]string)
		for k, v := range buildArgs {
			args.BuildArgs[k] = fmt.Sprintf("%v", v)
		}
	}

	if pushAfterBuild, ok := getBool(argsMap, "push_after_build"); ok {
		args.PushAfterBuild = pushAfterBuild
	}

	if registryURL, ok := getString(argsMap, "registry_url"); ok {
		args.RegistryURL = registryURL
	}

	// Execute the tool
	return tool.ExecuteBuild(ctx, args)
}

func (o *NoReflectToolOrchestrator) executePushImage(ctx context.Context, argsMap map[string]interface{}) (interface{}, error) {
	if o.toolFactory == nil {
		return nil, fmt.Errorf("tool factory not initialized")
	}

	tool := o.toolFactory.CreatePushImageTool()
	args := tools.AtomicPushImageArgs{}

	// Required fields
	if sessionID, ok := getString(argsMap, "session_id"); ok {
		args.SessionID = sessionID
	} else {
		return nil, fmt.Errorf("session_id is required")
	}

	if imageRef, ok := getString(argsMap, "image_ref"); ok {
		args.ImageRef = imageRef
	} else {
		return nil, fmt.Errorf("image_ref is required")
	}

	// Optional fields
	if registryURL, ok := getString(argsMap, "registry_url"); ok {
		args.RegistryURL = registryURL
	}

	if timeout, ok := getInt(argsMap, "timeout"); ok {
		args.Timeout = timeout
	}

	if retryCount, ok := getInt(argsMap, "retry_count"); ok {
		args.RetryCount = retryCount
	}

	if force, ok := getBool(argsMap, "force"); ok {
		args.Force = force
	}

	return tool.ExecutePush(ctx, args)
}

func (o *NoReflectToolOrchestrator) executePullImage(ctx context.Context, argsMap map[string]interface{}) (interface{}, error) {
	if o.toolFactory == nil {
		return nil, fmt.Errorf("tool factory not initialized")
	}

	tool := o.toolFactory.CreatePullImageTool()
	args := tools.AtomicPullImageArgs{}

	// Required fields
	if sessionID, ok := getString(argsMap, "session_id"); ok {
		args.SessionID = sessionID
	} else {
		return nil, fmt.Errorf("session_id is required")
	}

	if imageRef, ok := getString(argsMap, "image_ref"); ok {
		args.ImageRef = imageRef
	} else {
		return nil, fmt.Errorf("image_ref is required")
	}

	// Optional fields
	if timeout, ok := getInt(argsMap, "timeout"); ok {
		args.Timeout = timeout
	}

	if retryCount, ok := getInt(argsMap, "retry_count"); ok {
		args.RetryCount = retryCount
	}

	if force, ok := getBool(argsMap, "force"); ok {
		args.Force = force
	}

	return tool.Execute(ctx, args)
}

func (o *NoReflectToolOrchestrator) executeTagImage(ctx context.Context, argsMap map[string]interface{}) (interface{}, error) {
	if o.toolFactory == nil {
		return nil, fmt.Errorf("tool factory not initialized")
	}

	tool := o.toolFactory.CreateTagImageTool()
	args := tools.AtomicTagImageArgs{}

	// Required fields
	if sessionID, ok := getString(argsMap, "session_id"); ok {
		args.SessionID = sessionID
	} else {
		return nil, fmt.Errorf("session_id is required")
	}

	if sourceImage, ok := getString(argsMap, "source_image"); ok {
		args.SourceImage = sourceImage
	} else if imageRef, ok := getString(argsMap, "image_ref"); ok {
		// Support old field name for compatibility
		args.SourceImage = imageRef
	} else {
		return nil, fmt.Errorf("source_image is required")
	}

	if targetImage, ok := getString(argsMap, "target_image"); ok {
		args.TargetImage = targetImage
	} else if newTag, ok := getString(argsMap, "new_tag"); ok {
		// Support old field name for compatibility
		args.TargetImage = args.SourceImage + ":" + newTag
	} else {
		return nil, fmt.Errorf("target_image is required")
	}

	// Optional fields
	if force, ok := getBool(argsMap, "force"); ok {
		args.Force = force
	}

	return tool.ExecuteTag(ctx, args)
}

func (o *NoReflectToolOrchestrator) executeScanImageSecurity(ctx context.Context, argsMap map[string]interface{}) (interface{}, error) {
	if o.toolFactory == nil {
		return nil, fmt.Errorf("tool factory not initialized")
	}

	tool := o.toolFactory.CreateScanImageSecurityTool()
	args := tools.AtomicScanImageSecurityArgs{}

	// Required fields
	if sessionID, ok := getString(argsMap, "session_id"); ok {
		args.SessionID = sessionID
	} else {
		return nil, fmt.Errorf("session_id is required")
	}

	if imageName, ok := getString(argsMap, "image_name"); ok {
		args.ImageName = imageName
	} else if imageRef, ok := getString(argsMap, "image_ref"); ok {
		// Support old field name for compatibility
		args.ImageName = imageRef
	} else {
		return nil, fmt.Errorf("image_name is required")
	}

	// Optional fields
	if severityThreshold, ok := getString(argsMap, "severity_threshold"); ok {
		args.SeverityThreshold = severityThreshold
	}

	if vulnTypes, ok := argsMap["vuln_types"].([]interface{}); ok {
		args.VulnTypes = make([]string, len(vulnTypes))
		for i, v := range vulnTypes {
			args.VulnTypes[i] = fmt.Sprintf("%v", v)
		}
	}

	if includeFixable, ok := getBool(argsMap, "include_fixable"); ok {
		args.IncludeFixable = includeFixable
	}

	if maxResults, ok := getInt(argsMap, "max_results"); ok {
		args.MaxResults = maxResults
	}

	if includeRemediations, ok := getBool(argsMap, "include_remediations"); ok {
		args.IncludeRemediations = includeRemediations
	}

	if generateReport, ok := getBool(argsMap, "generate_report"); ok {
		args.GenerateReport = generateReport
	}

	if failOnCritical, ok := getBool(argsMap, "fail_on_critical"); ok {
		args.FailOnCritical = failOnCritical
	}

	return tool.Execute(ctx, args)
}

func (o *NoReflectToolOrchestrator) executeScanSecrets(ctx context.Context, argsMap map[string]interface{}) (interface{}, error) {
	if o.toolFactory == nil {
		return nil, fmt.Errorf("tool factory not initialized")
	}

	tool := o.toolFactory.CreateScanSecretsTool()
	args := tools.AtomicScanSecretsArgs{}

	// Required fields
	if sessionID, ok := getString(argsMap, "session_id"); ok {
		args.SessionID = sessionID
	} else {
		return nil, fmt.Errorf("session_id is required")
	}

	// Optional fields
	if scanPath, ok := getString(argsMap, "scan_path"); ok {
		args.ScanPath = scanPath
	}

	if filePatterns, ok := argsMap["file_patterns"].([]interface{}); ok {
		args.FilePatterns = make([]string, len(filePatterns))
		for i, v := range filePatterns {
			args.FilePatterns[i] = fmt.Sprintf("%v", v)
		}
	}

	if excludePatterns, ok := argsMap["exclude_patterns"].([]interface{}); ok {
		args.ExcludePatterns = make([]string, len(excludePatterns))
		for i, v := range excludePatterns {
			args.ExcludePatterns[i] = fmt.Sprintf("%v", v)
		}
	}

	if scanDockerfiles, ok := getBool(argsMap, "scan_dockerfiles"); ok {
		args.ScanDockerfiles = scanDockerfiles
	}

	if scanManifests, ok := getBool(argsMap, "scan_manifests"); ok {
		args.ScanManifests = scanManifests
	}

	if scanSourceCode, ok := getBool(argsMap, "scan_source_code"); ok {
		args.ScanSourceCode = scanSourceCode
	}

	if scanEnvFiles, ok := getBool(argsMap, "scan_env_files"); ok {
		args.ScanEnvFiles = scanEnvFiles
	}

	if suggestRemediation, ok := getBool(argsMap, "suggest_remediation"); ok {
		args.SuggestRemediation = suggestRemediation
	}

	if generateSecrets, ok := getBool(argsMap, "generate_secrets"); ok {
		args.GenerateSecrets = generateSecrets
	}

	return tool.Execute(ctx, args)
}

func (o *NoReflectToolOrchestrator) executeGenerateManifests(ctx context.Context, argsMap map[string]interface{}) (interface{}, error) {
	if o.toolFactory == nil {
		return nil, fmt.Errorf("tool factory not initialized")
	}

	tool := o.toolFactory.CreateGenerateManifestsTool()
	args := tools.AtomicGenerateManifestsArgs{}

	// Required fields
	if sessionID, ok := getString(argsMap, "session_id"); ok {
		args.SessionID = sessionID
	} else {
		return nil, fmt.Errorf("session_id is required")
	}

	if imageRef, ok := getString(argsMap, "image_ref"); ok {
		args.ImageRef = imageRef
	} else {
		return nil, fmt.Errorf("image_ref is required")
	}

	if appName, ok := getString(argsMap, "app_name"); ok {
		args.AppName = appName
	} else {
		return nil, fmt.Errorf("app_name is required")
	}

	// Optional fields
	if namespace, ok := getString(argsMap, "namespace"); ok {
		args.Namespace = namespace
	}

	if port, ok := getInt(argsMap, "port"); ok {
		args.Port = port
	}

	if replicas, ok := getInt(argsMap, "replicas"); ok {
		args.Replicas = replicas
	}

	if cpuRequest, ok := getString(argsMap, "cpu_request"); ok {
		args.CPURequest = cpuRequest
	}

	if memoryRequest, ok := getString(argsMap, "memory_request"); ok {
		args.MemoryRequest = memoryRequest
	}

	if cpuLimit, ok := getString(argsMap, "cpu_limit"); ok {
		args.CPULimit = cpuLimit
	}

	if memoryLimit, ok := getString(argsMap, "memory_limit"); ok {
		args.MemoryLimit = memoryLimit
	}

	if includeIngress, ok := getBool(argsMap, "include_ingress"); ok {
		args.IncludeIngress = includeIngress
	}

	if serviceType, ok := getString(argsMap, "service_type"); ok {
		args.ServiceType = serviceType
	}

	if environment, ok := argsMap["environment"].(map[string]interface{}); ok {
		args.Environment = make(map[string]string)
		for k, v := range environment {
			args.Environment[k] = fmt.Sprintf("%v", v)
		}
	}

	if secretHandling, ok := getString(argsMap, "secret_handling"); ok {
		args.SecretHandling = secretHandling
	}

	if secretManager, ok := getString(argsMap, "secret_manager"); ok {
		args.SecretManager = secretManager
	}

	if generateHelm, ok := getBool(argsMap, "generate_helm"); ok {
		args.GenerateHelm = generateHelm
	}

	if gitOpsReady, ok := getBool(argsMap, "gitops_ready"); ok {
		args.GitOpsReady = gitOpsReady
	}

	return tool.Execute(ctx, args)
}

func (o *NoReflectToolOrchestrator) executeDeployKubernetes(ctx context.Context, argsMap map[string]interface{}) (interface{}, error) {
	if o.toolFactory == nil {
		return nil, fmt.Errorf("tool factory not initialized")
	}

	tool := o.toolFactory.CreateDeployKubernetesTool()
	args := tools.AtomicDeployKubernetesArgs{}

	// Required fields
	if sessionID, ok := getString(argsMap, "session_id"); ok {
		args.SessionID = sessionID
	} else {
		return nil, fmt.Errorf("session_id is required")
	}

	if imageRef, ok := getString(argsMap, "image_ref"); ok {
		args.ImageRef = imageRef
	} else {
		return nil, fmt.Errorf("image_ref is required")
	}

	// Optional fields
	if appName, ok := getString(argsMap, "app_name"); ok {
		args.AppName = appName
	}

	if namespace, ok := getString(argsMap, "namespace"); ok {
		args.Namespace = namespace
	}

	if replicas, ok := getInt(argsMap, "replicas"); ok {
		args.Replicas = replicas
	}

	if port, ok := getInt(argsMap, "port"); ok {
		args.Port = port
	}

	if serviceType, ok := getString(argsMap, "service_type"); ok {
		args.ServiceType = serviceType
	}

	if includeIngress, ok := getBool(argsMap, "include_ingress"); ok {
		args.IncludeIngress = includeIngress
	}

	if environment, ok := argsMap["environment"].(map[string]interface{}); ok {
		args.Environment = make(map[string]string)
		for k, v := range environment {
			args.Environment[k] = fmt.Sprintf("%v", v)
		}
	}

	if cpuRequest, ok := getString(argsMap, "cpu_request"); ok {
		args.CPURequest = cpuRequest
	}

	if memoryRequest, ok := getString(argsMap, "memory_request"); ok {
		args.MemoryRequest = memoryRequest
	}

	if cpuLimit, ok := getString(argsMap, "cpu_limit"); ok {
		args.CPULimit = cpuLimit
	}

	if memoryLimit, ok := getString(argsMap, "memory_limit"); ok {
		args.MemoryLimit = memoryLimit
	}

	if generateOnly, ok := getBool(argsMap, "generate_only"); ok {
		args.GenerateOnly = generateOnly
	}

	if waitForReady, ok := getBool(argsMap, "wait_for_ready"); ok {
		args.WaitForReady = waitForReady
	}

	if waitTimeout, ok := getInt(argsMap, "wait_timeout"); ok {
		args.WaitTimeout = waitTimeout
	}

	if dryRun, ok := getBool(argsMap, "dry_run"); ok {
		args.DryRun = dryRun
	}

	return tool.Execute(ctx, args)
}

func (o *NoReflectToolOrchestrator) executeCheckHealth(ctx context.Context, argsMap map[string]interface{}) (interface{}, error) {
	if o.toolFactory == nil {
		return nil, fmt.Errorf("tool factory not initialized")
	}

	tool := o.toolFactory.CreateCheckHealthTool()
	args := tools.AtomicCheckHealthArgs{}

	// Required fields
	if sessionID, ok := getString(argsMap, "session_id"); ok {
		args.SessionID = sessionID
	} else {
		return nil, fmt.Errorf("session_id is required")
	}

	// Optional fields
	if namespace, ok := getString(argsMap, "namespace"); ok {
		args.Namespace = namespace
	}

	if appName, ok := getString(argsMap, "app_name"); ok {
		args.AppName = appName
	}

	if labelSelector, ok := getString(argsMap, "label_selector"); ok {
		args.LabelSelector = labelSelector
	}

	if includeServices, ok := getBool(argsMap, "include_services"); ok {
		args.IncludeServices = includeServices
	}

	if includeEvents, ok := getBool(argsMap, "include_events"); ok {
		args.IncludeEvents = includeEvents
	}

	if waitForReady, ok := getBool(argsMap, "wait_for_ready"); ok {
		args.WaitForReady = waitForReady
	}

	if waitTimeout, ok := getInt(argsMap, "wait_timeout"); ok {
		args.WaitTimeout = waitTimeout
	}

	if detailedAnalysis, ok := getBool(argsMap, "detailed_analysis"); ok {
		args.DetailedAnalysis = detailedAnalysis
	}

	if includeLogs, ok := getBool(argsMap, "include_logs"); ok {
		args.IncludeLogs = includeLogs
	}

	if logLines, ok := getInt(argsMap, "log_lines"); ok {
		args.LogLines = logLines
	}

	return tool.Execute(ctx, args)
}

func (o *NoReflectToolOrchestrator) executeGenerateDockerfile(ctx context.Context, argsMap map[string]interface{}) (interface{}, error) {
	if o.toolFactory == nil {
		return nil, fmt.Errorf("tool factory not initialized")
	}

	tool := o.toolFactory.CreateGenerateDockerfileTool()
	args := tools.GenerateDockerfileArgs{}

	// Required fields
	if sessionID, ok := getString(argsMap, "session_id"); ok {
		args.SessionID = sessionID
	} else {
		return nil, fmt.Errorf("session_id is required")
	}

	// Optional fields
	if baseImage, ok := getString(argsMap, "base_image"); ok {
		args.BaseImage = baseImage
	}

	if template, ok := getString(argsMap, "template"); ok {
		args.Template = template
	}

	if optimization, ok := getString(argsMap, "optimization"); ok {
		args.Optimization = optimization
	}

	if includeHealthCheck, ok := getBool(argsMap, "include_health_check"); ok {
		args.IncludeHealthCheck = includeHealthCheck
	}

	if buildArgs, ok := argsMap["build_args"].(map[string]interface{}); ok {
		args.BuildArgs = make(map[string]string)
		for k, v := range buildArgs {
			args.BuildArgs[k] = fmt.Sprintf("%v", v)
		}
	}

	if platform, ok := getString(argsMap, "platform"); ok {
		args.Platform = platform
	}

	return tool.Execute(ctx, args)
}

func (o *NoReflectToolOrchestrator) executeValidateDockerfile(ctx context.Context, argsMap map[string]interface{}) (interface{}, error) {
	if o.toolFactory == nil {
		return nil, fmt.Errorf("tool factory not initialized")
	}

	tool := o.toolFactory.CreateValidateDockerfileTool()
	args := tools.AtomicValidateDockerfileArgs{}

	// Required fields
	if sessionID, ok := getString(argsMap, "session_id"); ok {
		args.SessionID = sessionID
	} else {
		return nil, fmt.Errorf("session_id is required")
	}

	// Optional fields
	if dockerfilePath, ok := getString(argsMap, "dockerfile_path"); ok {
		args.DockerfilePath = dockerfilePath
	}

	if dockerfileContent, ok := getString(argsMap, "dockerfile_content"); ok {
		args.DockerfileContent = dockerfileContent
	}

	if useHadolint, ok := getBool(argsMap, "use_hadolint"); ok {
		args.UseHadolint = useHadolint
	}

	if severity, ok := getString(argsMap, "severity"); ok {
		args.Severity = severity
	}

	if ignoreRules, ok := argsMap["ignore_rules"].([]interface{}); ok {
		args.IgnoreRules = make([]string, len(ignoreRules))
		for i, v := range ignoreRules {
			args.IgnoreRules[i] = fmt.Sprintf("%v", v)
		}
	}

	if trustedRegistries, ok := argsMap["trusted_registries"].([]interface{}); ok {
		args.TrustedRegistries = make([]string, len(trustedRegistries))
		for i, v := range trustedRegistries {
			args.TrustedRegistries[i] = fmt.Sprintf("%v", v)
		}
	}

	if checkSecurity, ok := getBool(argsMap, "check_security"); ok {
		args.CheckSecurity = checkSecurity
	}

	if checkOptimization, ok := getBool(argsMap, "check_optimization"); ok {
		args.CheckOptimization = checkOptimization
	}

	if checkBestPractices, ok := getBool(argsMap, "check_best_practices"); ok {
		args.CheckBestPractices = checkBestPractices
	}

	if includeSuggestions, ok := getBool(argsMap, "include_suggestions"); ok {
		args.IncludeSuggestions = includeSuggestions
	}

	if generateFixes, ok := getBool(argsMap, "generate_fixes"); ok {
		args.GenerateFixes = generateFixes
	}

	return tool.Execute(ctx, args)
}
