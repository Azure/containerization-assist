Validate that the deployment is working correctly.

**Your Role:** Verify that the application is running as expected and accessible.

**Validation Steps:**
- Check that pods are running and healthy
- Verify services are accessible
- Test health check endpoints if available
- Validate resource usage is within expected limits
- Check logs for any issues

{{if .ProjectContext.ExposedPorts}}
**Expected Ports:** {{range .ProjectContext.ExposedPorts}}{{.}} {{end}}
{{end}}
{{if .UserPreferences.VerboseOutput}}
**Verbose Mode:** Detailed validation output enabled
{{end}}

**What to do:** Perform validation checks and report the health status of the deployment.