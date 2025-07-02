package orchestration

import (
	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/Azure/container-kit/pkg/mcp/internal/deploy"
	"github.com/Azure/container-kit/pkg/mcp/types/tools"
	"github.com/rs/zerolog"
)

// BuildRegistry provides type-safe registration and execution of build tools
type BuildRegistry = GenericRegistry[
	tools.Tool[build.DockerBuildParams, build.DockerBuildResult],
	build.DockerBuildParams,
	build.DockerBuildResult,
]

// DeployRegistry provides type-safe registration and execution of deploy tools
type DeployRegistry = GenericRegistry[
	tools.Tool[deploy.KubernetesDeployParams, deploy.KubernetesDeployResult],
	deploy.KubernetesDeployParams,
	deploy.KubernetesDeployResult,
]

// ScanRegistry provides type-safe registration and execution of security scan tools
type ScanRegistry = GenericRegistry[
	tools.Tool[deploy.SecurityScanParams, deploy.SecurityScanResult],
	deploy.SecurityScanParams,
	deploy.SecurityScanResult,
]

// SpecializedRegistries contains all domain-specific registries
type SpecializedRegistries struct {
	Build  *BuildRegistry
	Deploy *DeployRegistry
	Scan   *ScanRegistry
	logger zerolog.Logger
}

// NewSpecializedRegistries creates a new set of domain-specific registries
func NewSpecializedRegistries(logger zerolog.Logger) *SpecializedRegistries {
	return &SpecializedRegistries{
		Build:  NewGenericRegistry[tools.Tool[build.DockerBuildParams, build.DockerBuildResult], build.DockerBuildParams, build.DockerBuildResult](logger.With().Str("registry", "build").Logger()),
		Deploy: NewGenericRegistry[tools.Tool[deploy.KubernetesDeployParams, deploy.KubernetesDeployResult], deploy.KubernetesDeployParams, deploy.KubernetesDeployResult](logger.With().Str("registry", "deploy").Logger()),
		Scan:   NewGenericRegistry[tools.Tool[deploy.SecurityScanParams, deploy.SecurityScanResult], deploy.SecurityScanParams, deploy.SecurityScanResult](logger.With().Str("registry", "scan").Logger()),
		logger: logger.With().Str("component", "specialized_registries").Logger(),
	}
}

// RegisterBuildTools registers common build tools
func (sr *SpecializedRegistries) RegisterBuildTools(buildTool tools.Tool[build.DockerBuildParams, build.DockerBuildResult]) error {
	return sr.Build.Register("docker_build", buildTool)
}

// RegisterDeployTools registers common deploy tools
func (sr *SpecializedRegistries) RegisterDeployTools(deployTool tools.Tool[deploy.KubernetesDeployParams, deploy.KubernetesDeployResult]) error {
	return sr.Deploy.Register("kubernetes_deploy", deployTool)
}

// RegisterScanTools registers common security scan tools
func (sr *SpecializedRegistries) RegisterScanTools(scanTool tools.Tool[deploy.SecurityScanParams, deploy.SecurityScanResult]) error {
	return sr.Scan.Register("security_scan", scanTool)
}

// GetOverallStats returns statistics across all specialized registries
func (sr *SpecializedRegistries) GetOverallStats() OverallStats {
	buildStats := sr.Build.GetStats()
	deployStats := sr.Deploy.GetStats()
	scanStats := sr.Scan.GetStats()

	return OverallStats{
		TotalRegistries: 3,
		BuildTools:      buildStats.TotalTools,
		DeployTools:     deployStats.TotalTools,
		ScanTools:       scanStats.TotalTools,
		TotalTools:      buildStats.TotalTools + deployStats.TotalTools + scanStats.TotalTools,
		TotalUsage:      buildStats.TotalUsage + deployStats.TotalUsage + scanStats.TotalUsage,
	}
}

// OverallStats contains statistics across all registries
type OverallStats struct {
	TotalRegistries int
	BuildTools      int
	DeployTools     int
	ScanTools       int
	TotalTools      int
	TotalUsage      int64
}

// Additional specialized tool types

// PullRegistry for Docker pull operations
type PullRegistry = GenericRegistry[
	tools.Tool[build.DockerPullParams, build.DockerPullResult],
	build.DockerPullParams,
	build.DockerPullResult,
]

// PushRegistry for Docker push operations
type PushRegistry = GenericRegistry[
	tools.Tool[build.DockerPushParams, build.DockerPushResult],
	build.DockerPushParams,
	build.DockerPushResult,
]

// TagRegistry for Docker tag operations
type TagRegistry = GenericRegistry[
	tools.Tool[build.DockerTagParams, build.DockerTagResult],
	build.DockerTagParams,
	build.DockerTagResult,
]

// ExtendedBuildRegistries contains all build-related registries
type ExtendedBuildRegistries struct {
	Build  *BuildRegistry
	Pull   *PullRegistry
	Push   *PushRegistry
	Tag    *TagRegistry
	logger zerolog.Logger
}

// NewExtendedBuildRegistries creates registries for all Docker operations
func NewExtendedBuildRegistries(logger zerolog.Logger) *ExtendedBuildRegistries {
	return &ExtendedBuildRegistries{
		Build:  NewGenericRegistry[tools.Tool[build.DockerBuildParams, build.DockerBuildResult], build.DockerBuildParams, build.DockerBuildResult](logger.With().Str("registry", "build").Logger()),
		Pull:   NewGenericRegistry[tools.Tool[build.DockerPullParams, build.DockerPullResult], build.DockerPullParams, build.DockerPullResult](logger.With().Str("registry", "pull").Logger()),
		Push:   NewGenericRegistry[tools.Tool[build.DockerPushParams, build.DockerPushResult], build.DockerPushParams, build.DockerPushResult](logger.With().Str("registry", "push").Logger()),
		Tag:    NewGenericRegistry[tools.Tool[build.DockerTagParams, build.DockerTagResult], build.DockerTagParams, build.DockerTagResult](logger.With().Str("registry", "tag").Logger()),
		logger: logger.With().Str("component", "extended_build_registries").Logger(),
	}
}

// RegisterAllBuildTools registers a complete set of Docker build tools
func (ebr *ExtendedBuildRegistries) RegisterAllBuildTools(
	buildTool tools.Tool[build.DockerBuildParams, build.DockerBuildResult],
	pullTool tools.Tool[build.DockerPullParams, build.DockerPullResult],
	pushTool tools.Tool[build.DockerPushParams, build.DockerPushResult],
	tagTool tools.Tool[build.DockerTagParams, build.DockerTagResult],
) error {
	if err := ebr.Build.Register("docker_build", buildTool); err != nil {
		return err
	}
	if err := ebr.Pull.Register("docker_pull", pullTool); err != nil {
		return err
	}
	if err := ebr.Push.Register("docker_push", pushTool); err != nil {
		return err
	}
	if err := ebr.Tag.Register("docker_tag", tagTool); err != nil {
		return err
	}

	ebr.logger.Info().Msg("All Docker build tools registered successfully")
	return nil
}

// GetExtendedStats returns statistics for all build operations
func (ebr *ExtendedBuildRegistries) GetExtendedStats() ExtendedBuildStats {
	buildStats := ebr.Build.GetStats()
	pullStats := ebr.Pull.GetStats()
	pushStats := ebr.Push.GetStats()
	tagStats := ebr.Tag.GetStats()

	return ExtendedBuildStats{
		BuildTools: buildStats.TotalTools,
		PullTools:  pullStats.TotalTools,
		PushTools:  pushStats.TotalTools,
		TagTools:   tagStats.TotalTools,
		TotalTools: buildStats.TotalTools + pullStats.TotalTools + pushStats.TotalTools + tagStats.TotalTools,
		TotalUsage: buildStats.TotalUsage + pullStats.TotalUsage + pushStats.TotalUsage + tagStats.TotalUsage,
	}
}

// ExtendedBuildStats contains statistics for Docker build operations
type ExtendedBuildStats struct {
	BuildTools int
	PullTools  int
	PushTools  int
	TagTools   int
	TotalTools int
	TotalUsage int64
}
