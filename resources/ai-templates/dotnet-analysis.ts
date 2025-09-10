export const DOTNET_ANALYSIS_TEMPLATE = {
    id: "dotnet-analysis",
    name: "AI-Powered .NET Project Analysis",
    description: "Intelligent analysis of .NET projects with modern ecosystem awareness",
    version: "2.0.0",
    system: `You are a .NET ecosystem expert with comprehensive knowledge of:
- .NET Framework, .NET Core, .NET 5+, and their migration paths
- ASP.NET Core, Blazor, WPF, WinForms, and modern .NET frameworks
- NuGet package management and dependency analysis
- .NET containerization best practices and Docker optimizations
- Security considerations for .NET applications
- Performance tuning for different .NET workloads
- Cloud-native .NET development patterns

Stay current with .NET ecosystem trends and recommend modern approaches.`,
    user: `Analyze this .NET project for containerization:

**File Structure:**
{{file_list}}

**Project Files Content:**
{{project_files_content}}

**Configuration Files:**
{{config_files}}

**Directory Structure:**
{{directory_structure}}

**Current Date:** {{current_date}}

Provide comprehensive .NET analysis in JSON format:
{
  "dotnet_version": "detected_or_recommended_version",
  "target_framework": "net6.0|net7.0|net8.0|netframework4.8",
  "project_type": {
    "primary": "web|console|library|desktop|service|blazor",
    "framework": "aspnetcore|mvc|webapi|blazor|wpf|winforms|worker",
    "hosting_model": "kestrel|iis|selfhost|serverless",
    "modern_alternatives": ["suggestions_for_modernization"]
  },
  "build_system": {
    "sdk_style": true/false,
    "package_management": "packagereference|packagesconfig|paket",
    "build_optimizations": ["msbuild_improvements"],
    "containerization_features": ["docker_specific_optimizations"]
  },
  "dependencies": {
    "nuget_packages": ["essential_runtime_packages"],
    "framework_dependencies": ["framework_specific_deps"],
    "security_sensitive": ["packages_needing_security_attention"],
    "outdated": ["packages_that_should_be_updated"],
    "container_relevant": ["packages_affecting_containerization"]
  },
  "application_characteristics": {
    "startup_type": "fast|slow|lazy",
    "memory_profile": "low|medium|high",
    "cpu_profile": "light|moderate|intensive",
    "io_profile": "network|disk|both|minimal",
    "scaling_pattern": "horizontal|vertical|both",
    "state_management": "stateless|stateful|session"
  },
  "containerization_recommendations": {
    "base_image_preferences": ["ordered_list_of_dotnet_images"],
    "runtime_optimizations": {
      "gc_settings": "recommended_gc_config",
      "runtime_config": "dotnet_runtime_flags",
      "globalization": "invariant_mode_recommendations"
    },
    "multi_stage_strategy": "recommended_multi_stage_approach",
    "layer_optimization": ["strategies_for_dotnet_layer_caching"],
    "aot_compilation": "native_aot_recommendations"
  },
  "security_considerations": {
    "dotnet_security": [".net_specific_security_measures"],
    "dependency_security": ["nuget_security_concerns"],
    "runtime_security": ["runtime_security_recommendations"],
    "https_configuration": ["ssl_tls_recommendations"]
  },
  "performance_optimizations": {
    "build_time": ["faster_build_strategies"],
    "startup_time": ["faster_startup_strategies"],
    "runtime_performance": ["runtime_optimization_tips"],
    "memory_optimization": ["memory_usage_improvements"]
  },
  "cloud_native_features": {
    "configuration": ["appsettings_and_environment_config"],
    "logging": ["structured_logging_recommendations"],
    "health_checks": ["health_check_endpoints"],
    "metrics": ["telemetry_and_monitoring"],
    "service_discovery": ["service_mesh_integration"]
  },
  "migration_recommendations": {
    "framework_migration": "migration_path_if_applicable",
    "modernization_opportunities": ["code_modernization_suggestions"],
    "breaking_changes": ["potential_issues_to_address"]
  }
}`,
    outputFormat: "json",
    max_tokens: 4000,
    temperature: 0.1
} as const;