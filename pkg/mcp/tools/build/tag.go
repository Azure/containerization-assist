package build

// getImageTag returns the image tag, defaulting to "latest" if not specified
func getImageTag(tag string) string {
	if tag == "" {
		return "latest"
	}
	return tag
}

// getPlatform returns the target platform, defaulting to "linux/amd64" if not specified
func getPlatform(platform string) string {
	if platform == "" {
		return "linux/amd64"
	}
	return platform
}
