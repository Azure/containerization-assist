import type { AITemplate } from './types';

export const JVM_ANALYSIS: AITemplate = {
  id: 'jvm-analysis',
  name: 'AI-Powered JVM Project Analysis',
  description: 'Intelligent analysis of JVM-based projects with modern ecosystem awareness',
  version: '2.0.0',
  system: `You are a JVM ecosystem expert with comprehensive knowledge of:
- Modern Java, Kotlin, and Scala development practices
- Current framework versions and their characteristics
- Build system optimizations (Maven, Gradle, SBT)
- JVM containerization best practices
- Security considerations for JVM applications
- Performance tuning for different JVM workloads

Stay current with ecosystem trends and recommend modern approaches.`,
  user: 'Analyze this JVM project for containerization:\n\n**File Structure:**\n{{file_list}}\n\n**Configuration Files:**\n{{config_files}}\n\n**Build Files Content:**\n{{build_files_content}}\n\n**Directory Structure:**\n{{directory_structure}}\n\n**Current Date:** {{current_date}}\n\nProvide comprehensive JVM analysis in JSON format:\n{\n  "language": "java|kotlin|scala",\n  "jvm_version": "detected_or_recommended_version",\n  "framework": {\n    "primary": "main_framework_name",\n    "version": "framework_version",\n    "type": "web|batch|microservice|desktop|library",\n    "modern_alternatives": ["suggestions_for_modernization"]\n  },\n  "build_system": {\n    "type": "maven|gradle|sbt",\n    "version": "detected_version",\n    "optimization_opportunities": ["build_improvements"],\n    "containerization_plugins": ["recommended_plugins"]\n  },\n  "dependencies": {\n    "runtime": ["essential_runtime_deps"],\n    "security_sensitive": ["deps_that_need_security_attention"],\n    "outdated": ["deps_that_should_be_updated"],\n    "container_relevant": ["deps_that_affect_containerization"]\n  },\n  "application_characteristics": {\n    "startup_type": "fast|slow|lazy",\n    "memory_profile": "low|medium|high",\n    "cpu_profile": "light|moderate|intensive",\n    "io_profile": "network|disk|both|minimal",\n    "scaling_pattern": "horizontal|vertical|both"\n  },\n  "containerization_recommendations": {\n    "base_image_preferences": ["ordered_list_of_preferences"],\n    "jvm_tuning": {\n      "heap_settings": "recommended_heap_config",\n      "gc_settings": "recommended_gc_config",\n      "container_awareness": "jvm_container_flags"\n    },\n    "multi_stage_strategy": "recommended_multi_stage_approach",\n    "layer_optimization": ["strategies_for_layer_caching"]\n  },\n  "security_considerations": {\n    "jvm_security": ["jvm_specific_security_measures"],\n    "dependency_security": ["dependency_security_concerns"],\n    "runtime_security": ["runtime_security_recommendations"]\n  },\n  "performance_optimizations": {\n    "build_time": ["faster_build_strategies"],\n    "startup_time": ["faster_startup_strategies"],\n    "runtime_performance": ["runtime_optimization_tips"]\n  },\n  "health_monitoring": {\n    "health_endpoint": "recommended_health_check_endpoint",\n    "metrics_endpoints": ["observability_endpoints"],\n    "logging_recommendations": ["logging_best_practices"]\n  }\n}\n',
  outputFormat: 'json',
  max_tokens: 4000,
  temperature: 0.1,
} as const;
