# Container Kit Custom Chat Mode Guide

This guide explains how to set up and use the Container Kit custom chat mode in VS Code, which provides specialized AI assistance for Container Kit development and containerization workflows.

## Overview

The Container Kit custom chat mode transforms VS Code's Copilot Chat into a specialized assistant that understands:
- Container Kit's 4-layer architecture (API/Application/Domain/Infrastructure)
- CQRS pattern with command and query separation
- Event-driven architecture with domain events and saga orchestration
- The unified 10-step containerization workflow
- AI-powered error recovery patterns with ML optimization
- Security-first development practices
- Multi-language container support (16+ languages)

## Setup Instructions

### Prerequisites

- **VS Code** with GitHub Copilot extension installed and activated
- **Container Kit repository** cloned locally
- **GitHub Copilot subscription** (required for chat functionality)

### Installing the Custom Chat Mode

#### Option 1: Using Command Palette (Recommended)

1. **Open Container Kit in VS Code**
   ```bash
   cd /path/to/container-kit
   code .
   ```

2. **Open Command Palette**
   - macOS: `⇧ ⌘ P`
   - Windows/Linux: `Ctrl + Shift + P`

3. **Create Chat Mode File**
   - Type: `Chat: New Mode File`
   - Select: **"Workspace"** (this will create in `.github/chatmodes/`)
   - Name: `container-kit`

4. **Replace Content**
   - VS Code will create `container-kit.chatmode.md`
   - Replace the generated content with our custom Container Kit mode
   - The file should already exist at `.github/chatmodes/container-kit.chatmode.md`

#### Option 2: Manual File Creation

The custom chat mode file is already created in this repository at:
```
.github/chatmodes/container-kit.chatmode.md
```

If you need to recreate it:

1. Create the directory: `mkdir -p .github/chatmodes`
2. Copy the provided `container-kit.chatmode.md` file
3. Restart VS Code to recognize the new mode

### Activating the Chat Mode

1. **Open Copilot Chat**
   - macOS: `⌃ ⌘ I`
   - Windows/Linux: `Ctrl + Alt + I`

2. **Select Container Kit Mode**
   - Click the chat mode dropdown (should show "General" by default)
   - Select **"Container Kit Development Assistant"**
   - You'll see the description: "AI-powered containerization assistant for Container Kit development and workflow guidance"

3. **Verify Activation**
   - The chat input should now show the Container Kit mode active
   - The AI assistant will now follow Container Kit-specific guidelines

## Using the Custom Chat Mode

### Core Capabilities

The Container Kit chat mode provides specialized assistance in these areas:

#### 🏗️ Architecture Guidance
Ask questions about Container Kit's 4-layer architecture:

```
How should I implement a new workflow step using CQRS patterns?
```

```
Where does session management belong in the architecture?
```

```
How do I maintain clean dependencies between layers?
```

```
How do I implement domain events for workflow coordination?
```

```
What's the best pattern for saga orchestration in complex workflows?
```

#### 🔄 Workflow Development
Get help with the 10-step containerization process:

```
How do I add progress tracking to step 5 (Load Image)?
```

```
What's the best way to implement AI retry logic for the Build step?
```

```
How do I handle errors in the Kubernetes deployment step?
```

```
How do I implement ML-powered build optimization?
```

```
What's the pattern for event-driven workflow coordination?
```

#### 🛠️ Technology Integration
Assistance with Container Kit's tech stack:

```
How do I add a new MCP tool to the registry?
```

```
What's the best pattern for Azure OpenAI integration?
```

```
How do I implement BoltDB session persistence?
```

```
How do I integrate with the event publishing system?
```

```
What's the pattern for implementing CQRS command handlers?
```

```
How do I add observability with distributed tracing?
```

#### 🔧 Code Quality & Testing
Development best practices specific to Container Kit:

```
How do I use the Rich error system from pkg/mcp/domain/errors?
```

```
What testing patterns should I follow for workflow steps?
```

```
How do I add a new language template?
```

```
What's the pattern for property-based testing in Container Kit?
```

```
How do I implement event handler testing with mocks?
```

### Example Conversations

#### Adding a New Workflow Step

**You:** *"I want to add a new step between Build and Scan for image optimization. How should I implement this?"*

**Container Kit Assistant:** *The assistant will provide guidance on:*
- Where to place the step in the Infrastructure layer (`pkg/mcp/infrastructure/steps/`)
- How to integrate with the workflow progress tracking and event system
- Error handling patterns using Rich error system from `pkg/mcp/domain/errors/`
- Integration with the unified orchestrator and step execution framework
- AI-powered error recovery and retry logic patterns
- Testing strategies including workflow integration tests
- ML integration for build optimization and pattern recognition

#### Debugging a Deployment Issue

**You:** *"Step 9 (Deploy) is failing with pod crash loops. How do I debug this?"*

**Container Kit Assistant:** *The assistant will help with:*
- Checking the AI-powered manifest fixing capabilities
- Reviewing Kubernetes health check patterns
- Understanding the error context system
- Implementing better error recovery strategies
- Using the structured logging for debugging

#### Adding Language Support

**You:** *"How do I add support for Scala containerization?"*

**Container Kit Assistant:** *The assistant will guide you through:*
- Creating a new Dockerfile template in `templates/dockerfiles/dockerfile-scala/`
- Updating repository analysis patterns
- Testing with Scala project samples
- Ensuring security scanning compatibility
- Documenting the new language support

### Advanced Usage Tips

#### 1. Provide Context
When asking questions, include relevant context:

```
I'm working on pkg/mcp/infrastructure/steps/build.go and need to add 
Docker BuildKit support. How should I integrate this with the existing 
AI retry logic?
```

#### 2. Reference Architecture Layers
Specify which layer you're working with:

```
In the Domain layer, how do I extend the workflow types to support 
custom deployment strategies?
```

#### 3. Include Error Messages
When debugging, share the actual error:

```
I'm getting this RichError in the scan step: "scanner_unavailable". 
How do I implement fallback scanning with the unified security scanner?
```

#### 4. Ask for Code Reviews
Request architectural feedback:

```
Can you review this session manager implementation? Does it follow 
Container Kit's clean architecture principles?
```

## Chat Mode Features

### Enabled Tools
The Container Kit chat mode has access to:

- **`codebase`** - Full codebase understanding and navigation
- **`search`** - Intelligent code search across the repository
- **`findTestFiles`** - Locate and analyze test patterns
- **`githubRepo`** - Repository metadata and history
- **`usages`** - Find usage patterns across the codebase
- **`bash`** - Execute Container Kit commands (`make build`, `make test`, etc.)
- **`terminal`** - Full terminal access for development workflows

### AI Model
- **Claude Sonnet 4** - Optimized for complex software architecture discussions
- Enhanced with Container Kit domain knowledge
- Understands MCP protocol patterns and containerization workflows

## Troubleshooting

### Chat Mode Not Appearing

1. **Check File Location**
   ```bash
   ls -la .github/chatmodes/
   # Should show: container-kit.chatmode.md
   ```

2. **Verify File Format**
   - Ensure the file has proper YAML front-matter
   - Check that the description field is properly quoted
   - Validate the tools array syntax

3. **Restart VS Code**
   ```bash
   # Close VS Code completely, then reopen
   code .
   ```

4. **Check VS Code Settings**
   - Open Settings (`⌘ ,` or `Ctrl + ,`)
   - Search for: `chat.modeFilesLocations`
   - Ensure it includes `.github/chatmodes` (default)

### Chat Mode Active But Not Working

1. **Verify Chat Mode Selection**
   - Look for "Container Kit Development Assistant" in the dropdown
   - The description should mention "AI-powered containerization assistant"

2. **Test with Simple Query**
   ```
   What are the 10 steps in Container Kit's containerize_and_deploy workflow?
   ```

3. **Check Copilot Status**
   - Ensure GitHub Copilot is signed in and active
   - Verify you have chat access (not just code completion)

### Performance Issues

1. **Large Codebase**
   - The Container Kit codebase is substantial (~150+ files)
   - Initial queries may take longer as the AI analyzes the code
   - Subsequent queries should be faster

2. **Optimize Queries**
   - Be specific about which files or directories you're working with
   - Reference specific architectural layers to narrow scope

## Best Practices

### 1. Start with Architecture
Always begin conversations by establishing the architectural context:

```
I'm working on the Infrastructure layer, specifically with Docker integration. 
How do I...
```

### 2. Use Container Kit Terminology
Leverage the domain-specific language:

- "workflow steps" instead of "functions"
- "progress tracking" instead of "status updates"  
- "AI retry logic" instead of "error handling"
- "Rich error system" instead of "exceptions"
- "domain events" instead of "notifications"
- "command handlers" instead of "controllers"
- "saga orchestration" instead of "transaction management"
- "ML optimization" instead of "performance tuning"

### 3. Reference Documentation
The chat mode is aware of Container Kit's extensive documentation:

```
According to ADR-006 (Four-Layer MCP Architecture), how should I implement 
this new feature in the 4-layer architecture?
```

### 4. Think in Workflows
Frame questions around the 10-step containerization process:

```
How do I add custom validation between the Scan step (9) and Finalize step (10)?
```

### 5. Security First
Always consider security implications:

```
What security scanning patterns should I follow when adding this new 
container registry integration?
```

## Integration with Development Workflow

### Daily Development
Use the chat mode for:
- **Architecture decisions** - "Should this be in Domain or Infrastructure layer?"
- **Code reviews** - "Does this follow Container Kit patterns?"
- **Debugging** - "Why is the workflow session not persisting?"
- **Testing strategies** - "How do I test this MCP integration?"

### Feature Development
Leverage for:
- **Planning** - "How do I add support for Podman alongside Docker?"
- **Implementation** - "What's the pattern for adding AI analysis to this step?"
- **Integration** - "How do I ensure this works with existing error recovery?"
- **Documentation** - "What ADR should I write for this change?"

### Maintenance
Get help with:
- **Performance optimization** - "How do I improve workflow step performance?"
- **Security updates** - "How do I update vulnerability scanning patterns?"
- **Dependency management** - "How do I upgrade the mcp-go library safely?"
- **Technical debt** - "What refactoring opportunities exist in this layer?"

## Advanced Configuration

### Custom Mode Locations
If you prefer a different location for chat modes:

1. **Open VS Code Settings**
2. **Search for:** `chat.modeFilesLocations`
3. **Add custom path:** `["docs/chatmodes", ".github/chatmodes"]`
4. **Move the file** to your preferred location
5. **Restart VS Code**

### Mode Variations
You can create specialized variations:

- **`container-kit-security.chatmode.md`** - Focus on security scanning and vulnerability analysis
- **`container-kit-k8s.chatmode.md`** - Specialized for Kubernetes manifest generation
- **`container-kit-ai.chatmode.md`** - Focus on AI integration and error recovery patterns

### Team Sharing
To share the chat mode with your team:

1. **Commit the file** to the repository:
   ```bash
   git add .github/chatmodes/container-kit.chatmode.md
   git commit -m "Add Container Kit custom chat mode"
   ```

2. **Document in README** or development guidelines
3. **Include in onboarding** process for new developers

## Conclusion

The Container Kit custom chat mode transforms VS Code into a specialized development environment for containerization workflows. By providing domain-specific knowledge and architectural guidance, it helps developers:

- Maintain clean architecture principles
- Implement robust error recovery patterns
- Follow security best practices
- Leverage AI-powered automation effectively
- Build scalable containerization solutions

For additional help:
- Review the [Container Kit Design Document](CONTAINER_KIT_DESIGN_DOCUMENT.md)
- Check [Architectural Decision Records](architecture/adr/)
- Consult the [New Developer Guide](NEW_DEVELOPER_GUIDE.md)
- Use `make help` for available commands