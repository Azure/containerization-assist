# Change Log

## [1.3.2]

- add MCP prompts for local Kind and AKS loop
- add install badges in README for VSCode and VSCode Insiders Marketplace

## [1.3.1]

- fix: sync mcpName casing to match MCP Registry namespace (Azure, not azure) (#608)

## [1.3.0]

- add vulnerability remediation recommendations to scan-image tool output (#587)
- convert build-image to context providing tool (#571)
- broaden socket auto-detection for Rancher, OrbStack, Podman, and DOCKER_HOST (#602)
- mcp registry publish support (#605)
- dependency updates (#591, #592, #593, #596, #598, #599, #601)

## [1.2.0]

- export tool constants separately from implementation (#581)
- add embedded knowledge pack exports (#579)
- dependency updates: @modelcontextprotocol/sdk, @isaacs/brace-expansion, and GitHub Actions workflows (#580, #578, #575, #576, #582, #584, #585, #586)


## [1.2.0-dev.1]

- export tool constants separately from implementation (#581)
- add embedded knowledge pack exports (#579)
- dependency updates: @modelcontextprotocol/sdk, @isaacs/brace-expansion, and GitHub Actions workflows (#580, #578, #575, #576, #582, #584, #585, #586)

## [1.1.0]

- add new `osv` parser for scan-image tool that has no CLI dependencies
- bug fix for windows docker named pipe socket connection
- update description of `analyze-repo` tool
- add policy integration test for `fix-dockerfile` tool
- show progress notifications during docker build operations
- dependency updates
- remove "additionalProperties" field from generated vscode package.json
- fix scan-image tool schema to use 'grype'/'snyk' values
- bump @modelcontextprotocol/sdk from 1.24.1 to 1.25.2 to address high severity [CVE-2026-0621](https://github.com/advisories/GHSA-8r9q-7v3j-jr4g)
- VSCode integration
- Separate mcp sdk from base tools

## [1.1.0-dev.4]

- add new `osv` parser for scan-image tool that has no CLI dependencies
- bug fix for windows docker named pipe socket connection

## [1.1.0-dev.3]

- update description of `analyze-repo` tool
- add policy integration test for `fix-dockerfile` tool
- show progress notifications during docker build operations
- dependency updates

## [1.1.0-dev.2]

- remove "additionalProperties" field from generated vscode package.json
- fix scan-image tool schema to use 'grype'/'snyk' values
- bump @modelcontextprotocol/sdk from 1.24.1 to 1.25.2 

## [1.1.0-dev.1]

- VSCode integration
- Separate mcp sdk from base tools

## [1.0.2]

- Upgraded modelcontextprotocol/sdk to v1.24.1 to address high CVE [GHSA-w48q-cv73-mx4w](https://github.com/advisories/GHSA-w48q-cv73-mx4w)

## [1.0.1]

- Enhanced output format options
- Improved policy management features
- Added support for cross-platform Docker builds
- Fixed bugs related to local registry validation
- Fixed bugs loading built-in knowledge packs

## [1.0.1-dev.6]

- Dev Release with bug fixes, local registry validation, cross-platform docker build support, and .dockerignore file parsing

## [1.0.1-dev.5]

- Dev Release improved telemetry

## [1.0.1-dev.4]

- Dev Release with updated tool output formatting

## [1.0.1-dev.3]

- Dev Release with updated tool contracts

## [1.0.1-dev.2]

- Dev Release with doc and dep updates

## [1.0.1-dev.1]

- Dev Release with updated output formatting and policy updates

## [1.0.0]

- Initial release
