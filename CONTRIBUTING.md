# Contributor License Agreement

This project welcomes contributions and suggestions. Most contributions require you to
agree to a Contributor License Agreement (CLA) declaring that you have the right to,
and actually do, grant us the rights to use your contribution. For details, visit
https://cla.microsoft.com.

When you submit a pull request, a CLA-bot will automatically determine whether you need
to provide a CLA and decorate the PR appropriately (e.g., label, comment). Simply follow the
instructions provided by the bot. You will only need to do this once across all repositories using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/)
or contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

## Development Setup

### Policy Development (Source Installation)

When working from a cloned copy of the repo, you can test policy changes directly:

```bash
# Create a project-local policy directory
mkdir -p .containerization-assist/policy

# Copy an example policy from the repo
cp policies.user.examples/allow-all-registries.rego .containerization-assist/policy/

# Or create your own from the template
cp policies.user.examples/custom-organization-template.rego .containerization-assist/policy/my-policy.rego

# Policies are auto-reloaded on the next tool execution — no restart needed
```

The `policies.user.examples/` directory contains ready-to-use examples:

| Example | Purpose |
|---------|---------|
| `allow-all-registries.rego` | Override MCR base image preference |
| `warn-only-mode.rego` | Convert all violations to warnings |
| `custom-organization-template.rego` | Template for org-specific rules |

### Testing Policies

```bash
# Validate policy syntax
opa check .containerization-assist/policy/my-policy.rego

# Run policy tests
opa test .containerization-assist/policy/

# List discovered policies
npx containerization-assist-mcp list-policies

# Show merged policy result
npx containerization-assist-mcp list-policies --show-merged
```
