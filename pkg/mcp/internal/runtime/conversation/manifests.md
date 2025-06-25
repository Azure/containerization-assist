Generate Kubernetes manifests for the containerized application.

**Your Role:** Create appropriate Kubernetes resources (Deployment, Service, ConfigMap, etc.) based on the application requirements.

**Considerations:**
- Create a Deployment with appropriate resource limits
- Add a Service if the application exposes ports
- Include ConfigMaps or Secrets for configuration
- Follow Kubernetes best practices for labels and selectors
- Consider the user's preferred namespace and registry

{{if .ProjectContext.ExposedPorts}}
**Detected Ports:** {{range .ProjectContext.ExposedPorts}}{{.}} {{end}}
{{end}}
{{if .UserPreferences.PreferredNamespace}}
**Target Namespace:** {{.UserPreferences.PreferredNamespace}}
{{end}}

**What to do:** Use the generate_manifests_atomic tool to create the Kubernetes manifests.