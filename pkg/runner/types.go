package runner

// ManifestDeployResult stores the result of a single manifest deployment
type ManifestDeployResult struct {
	Path    string
	Success bool
	Output  string
}

