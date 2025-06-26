Generate an optimized Dockerfile based on the repository analysis.

**Your Role:** Create a Dockerfile that follows best practices for the detected project type.

**Considerations:**
- Use appropriate base images for the detected language/framework
- Follow multi-stage build patterns when beneficial
- Optimize for the user's preferred optimization strategy
- Include health checks if appropriate
- Handle build arguments and environment variables properly

{{if .ProjectContext.Language}}
**Detected Language:** {{.ProjectContext.Language}}
{{end}}
{{if .ProjectContext.Framework}}
**Detected Framework:** {{.ProjectContext.Framework}}
{{end}}
{{if .ProjectContext.ExposedPorts}}
**Detected Ports:** {{range .ProjectContext.ExposedPorts}}{{.}} {{end}}
{{end}}

**What to do:** Use the generate_dockerfile_atomic tool to create an optimized Dockerfile and explain your choices.