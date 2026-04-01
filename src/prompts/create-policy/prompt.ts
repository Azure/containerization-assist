/**
 * create-containerization-policy tool prompt
 *
 * Returns guidance text that instructs the LLM to walk the user through
 * creating a custom OPA Rego policy. The output includes a suggested
 * step-by-step plan (rendered as a checklist/todo in VS Code Copilot Chat)
 * and each step has a recommended default the user can accept or override.
 *
 * The first example policy is a base image allowlist.
 */

import { POLICY_PROJECT_DIR, POLICY_SUBDIR, POLICY_GLOBAL_APP_NAME } from '@/config/constants';
import { TOOL_NAME } from '@/tools';

export function buildCreatePolicyPrompt(): string {
  const projectDir = `\`<workspace>/${POLICY_PROJECT_DIR}/${POLICY_SUBDIR}/\``;
  const globalDir = `\`~/.config/${POLICY_GLOBAL_APP_NAME}/${POLICY_SUBDIR}/\``;

  return `You are helping the user create a custom **containerization-assist OPA Rego policy**.

## VS Code Integration

If you are running in VS Code Copilot Chat, leverage the built-in tools for the best integrated experience:

- **\`manage_todo_list\`**: Use this tool to create and manage a checklist for the policy creation steps below. Create the todo list with all 7 steps at the start, then update each item's status as you work through them (pending → in-progress → done). This gives the user a visual progress tracker in the chat.
- **\`vscode_askQuestions\`**: Use this tool to present choices to the user (e.g., severity level, category, storage location) as interactive selection prompts instead of plain text questions. This provides a clickable UI for the user to choose from.

If these tools are not available (e.g., non-VS Code client), fall back to presenting the plan as a numbered checklist and asking questions as plain text.

## Suggested Plan

Present this plan to the user as a numbered checklist so they can see the full process upfront. Then work through each step one at a time. If \`manage_todo_list\` is available, create the todo list with these 7 items immediately.

1. Choose what the policy enforces (e.g., base image allowlist)
2. Set the severity level (block, warn, or suggest)
3. Pick the policy category (security, quality, performance, compliance)
4. Name the policy file
5. Choose where to store the policy (project or global)
6. Generate and write the policy file
7. Verify the policy is loaded

## Step-by-Step Instructions

Walk through each step below **one at a time**. For each step, present the **recommended default** and ask the user to accept it or provide their own preference. If using \`vscode_askQuestions\`, present the options as an interactive selection with the recommended default pre-selected. If the user simply agrees or says "yes" / "looks good" / "next", use the recommended default and move on.

### Step 1: What should this policy do?

**Recommended default**: Create a **base image allowlist** — only allow Dockerfiles that use approved base images from a trusted set of registries/images.

Present this default and ask the user to accept it or describe a different policy. Other examples:

**Container image (Dockerfile) policies:**
- Warn when no HEALTHCHECK is present
- Require specific labels (team, version, cost-center)
- Restrict certain base images or OS versions
- Block images running as root
- Enforce a non-root USER instruction

**Kubernetes manifest policies:**
- Enforce resource limits and requests on all containers
- Block privileged containers or host networking
- Require specific labels or annotations on deployments
- Restrict allowed namespaces
- Enforce readiness and liveness probes

### Step 2: What severity should violations have?

**Recommended default**: \`block\` — Prevents generation/build (hard enforcement).

Present the options with the recommended one marked:
- **\`block\` (recommended)** — Prevents generation/build (hard enforcement)
- \`warn\` — Shows a warning but allows the operation to proceed
- \`suggest\` — Advisory recommendation, lowest priority

### Step 3: What category does this policy belong to?

**Recommended default**: \`security\` — Security-related rules (priority 90-100).

Present the options with the recommended one marked:
- **\`security\` (recommended)** — Security-related rules (priority 90-100)
- \`quality\` — Code quality and best practices (priority 70-89)
- \`performance\` — Performance optimization (priority 50-69)
- \`compliance\` — Organizational compliance (priority 30-49)

### Step 4: What should the policy be named?

**Recommended default**: Suggest a name based on the user's answers from Step 1. For the base image allowlist default, suggest \`approved-base-images.rego\`. The filename should be kebab-case with a \`.rego\` extension.

### Step 5: Where should the policy be stored?

**Recommended default**: **Project directory** — tracked in git, applies only to this project.

Present two options:
- **Project (recommended)**: ${projectDir} — tracked in git, applies only to this project
- Global: ${globalDir} — applies to all projects for this user

If the user agrees, use the project directory. Create the directory if it does not exist.

### Step 6: Generate the policy

Using the user's choices (or defaults), generate a complete, working \`.rego\` policy file and write it to the chosen directory.

### Step 7: Verify

After writing the file:
1. Remind the user the policy takes effect on the next tool execution (no restart needed — policies are re-discovered automatically, including content changes).
2. Suggest testing with: \`opa check <policy-file>\` to validate syntax.
3. Suggest running the **\`${TOOL_NAME.OPS}\`** tool with action \`status\` to confirm the policy appears in the loaded policies list.

## Base Image Allowlist Example

If the user accepts the default in Step 1 (or requests a base image allowlist), use this as the starting template. Adjust the allowed images list based on the user's needs:

\`\`\`rego
package containerization.approved_base_images

# Approved base images — adjust this list to match your organization's requirements
approved_images := [
  "mcr.microsoft.com/",
]

# Extract all FROM lines from the Dockerfile
from_lines := [line |
  line := split(input.content, "\\n")[_]
  startswith(trim_space(line), "FROM ")
]

# Extract the image reference from a FROM line (handles "FROM image" and "FROM image AS stage")
image_from_line(line) := image if {
  parts := split(trim_space(line), " ")
  image := parts[1]
}

# Check if an image matches any approved prefix
is_approved(image) if {
  some prefix in approved_images
  startswith(image, prefix)
}

# Flag each unapproved FROM line
violations contains result if {
  input_type == "dockerfile"
  some line in from_lines
  image := image_from_line(line)
  not is_approved(image)

  result := {
    "rule": "approved-base-images",
    "category": "security",
    "priority": 95,
    "severity": "block",
    "message": sprintf("Base image '%s' is not in the approved list. Approved prefixes: %v", [image, approved_images]),
    "description": "Only approved base images should be used for security and supply-chain integrity",
  }
}

# Input type detection
is_dockerfile if contains(input.content, "FROM ")
input_type := "dockerfile" if is_dockerfile else := "unknown"

# Policy decision
default allow := false
allow if count(violations) == 0

# Result structure (required)
result := {
  "allow": allow,
  "violations": violations,
  "warnings": set(),
  "suggestions": set(),
  "summary": {
    "total_violations": count(violations),
    "total_warnings": 0,
    "total_suggestions": 0,
  },
}
\`\`\`

## Rego Policy Reference

For custom policies (when the user does NOT want the base image allowlist), use this structure:

### Required Structure

\`\`\`rego
package containerization.<policy_namespace>

# Blocking violations (severity: "block")
violations contains result if {
  input_type == "dockerfile"    # or "kubernetes", "compose"
  # ... your condition ...

  result := {
    "rule": "<rule-id>",           # kebab-case identifier
    "category": "<category>",      # security | quality | performance | compliance
    "priority": <number>,          # 90-100 security, 70-89 quality, 50-69 performance, 30-49 compliance
    "severity": "block",           # block | warn | suggest
    "message": "<user-facing message>",
    "description": "<brief rule description>",
  }
}

# Non-blocking warnings (severity: "warn")
warnings contains result if {
  input_type == "dockerfile"
  # ... your condition ...

  result := {
    "rule": "<rule-id>",
    "category": "<category>",
    "priority": <number>,
    "severity": "warn",
    "message": "<user-facing message>",
    "description": "<brief rule description>",
  }
}

# Advisory suggestions (severity: "suggest")
suggestions contains result if {
  input_type == "dockerfile"
  # ... your condition ...

  result := {
    "rule": "<rule-id>",
    "category": "<category>",
    "priority": <number>,
    "severity": "suggest",
    "message": "<user-facing message>",
    "description": "<brief rule description>",
  }
}

# Input type detection
is_dockerfile if contains(input.content, "FROM ")
input_type := "dockerfile" if is_dockerfile else := "unknown"

# Policy decision
default allow := false
allow if count(violations) == 0

# Result structure (required)
result := {
  "allow": allow,
  "violations": violations,
  "warnings": warnings,
  "suggestions": suggestions,
  "summary": {
    "total_violations": count(violations),
    "total_warnings": count(warnings),
    "total_suggestions": count(suggestions),
  },
}
\`\`\`

### Key Rules for Valid Policies

- The package name **must** start with \`containerization.\` (e.g., \`containerization.my_custom_rules\`)
- Use \`input.content\` to access the raw file content being evaluated
- Use \`input_type\` to detect what kind of file is being evaluated (\`"dockerfile"\`, etc.)
- The \`result\` object at the bottom is **required** — it is how the policy system reads outcomes
- Each rule result **must** include all 6 fields: \`rule\`, \`category\`, \`priority\`, \`severity\`, \`message\`, \`description\`
- Use Rego's \`regex.match()\` for pattern matching, \`contains()\` for substring checks
- Only include the rule sets that apply (e.g., skip \`suggestions\` if there are none, but still include it as an empty set in the result)

### Priority Ranges

| Range   | Category    | Examples                                    |
|---------|-------------|---------------------------------------------|
| 90–100  | Security    | Root user, secrets in env, privileged mode   |
| 70–89   | Quality     | Missing HEALTHCHECK, :latest tag, no labels  |
| 50–69   | Performance | Layer optimization, multi-stage builds       |
| 30–49   | Compliance  | Required labels, approved registries         |

### Common Rego Patterns

**Check for a pattern in Dockerfile:**
\`\`\`rego
regex.match(\`(?m)^USER\\s+root\`, input.content)
\`\`\`

**Check a pattern is NOT present:**
\`\`\`rego
not regex.match(\`HEALTHCHECK\`, input.content)
\`\`\`

**Extract and iterate over FROM lines:**
\`\`\`rego
from_lines := [line |
  line := split(input.content, "\\n")[_]
  startswith(trim_space(line), "FROM ")
]
some line in from_lines
# ... check each line ...
\`\`\`

**Check for label existence:**
\`\`\`rego
regex.match(\`(?mi)^LABEL\\s+.*team\\s*=\`, input.content)
\`\`\`

## Important Rules

- Present the plan as a numbered checklist first, then work through steps one at a time.
- If \`manage_todo_list\` is available, create the checklist as a managed todo list and update item status as you complete each step.
- If \`vscode_askQuestions\` is available, use it for all choice-based steps (severity, category, storage location) to provide interactive selection UI.
- For each step, show the **recommended default** prominently and ask the user to accept or customize.
- If the user simply agrees (e.g., "yes", "looks good", "next", "ok"), use the default and move on.
- Generate a **complete, working** Rego file — no placeholders or TODOs.
- Use the user's answers to determine which rule sets to include (violations, warnings, suggestions).
- Match the severity to the user's intent: enforcement → \`block\`, advisory → \`warn\`, recommendation → \`suggest\`.
- Include helpful comments in the generated policy explaining what each rule does.
- If the user's request is ambiguous, ask a clarifying follow-up before generating code.`;
}
