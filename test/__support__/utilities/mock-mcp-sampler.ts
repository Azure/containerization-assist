/**
 * Mock MCP Sampler for testing
 */

import { Success as ok, Failure as fail, type Result } from "../../src/types"

export class MockMCPSampler {
  private mockResponses: Map<string, any> = new Map();
  private callLog: Array<{ templateId: string; variables: any }> = [];

  addMockResponse(templateId: string, response: any): void {
    this.mockResponses.set(templateId, response);
  }

  async sample<T>(request: {
    templateId: string;
    variables: Record<string, any>;
    format: 'json' | 'text';
  }): Promise<Result<T> & { success: boolean; content?: T; error?: { message: string } }> {
    this.callLog.push({
      templateId: request.templateId,
      variables: request.variables
    });

    const mockResponse = this.mockResponses.get(request.templateId);
    
    if (mockResponse) {
      return {
        success: true,
        content: mockResponse as T,
        ...ok(mockResponse as T)
      };
    }

    // Default fallback responses for common templates
    const defaultResponses: Record<string, any> = {
      'repository-analysis': {
        language: 'csharp',
        languageVersion: '8.0',
        framework: 'aspnetcore',
        frameworkVersion: 'net8.0',
        buildSystem: {
          type: 'msbuild',
          buildFile: 'Web.csproj',
          buildCommand: 'dotnet build',
          testCommand: 'dotnet test'
        },
        dependencies: ['Microsoft.AspNetCore.App'],
        devDependencies: [],
        entryPoint: 'Program.cs',
        suggestedPorts: [80, 443, 5000],
        dockerConfig: {
          baseImage: 'mcr.microsoft.com/dotnet/aspnet:8.0',
          multistage: true,
          nonRootUser: true
        }
      },
      'base-image-resolution': {
        primary_recommendation: {
          image: 'mcr.microsoft.com/dotnet/aspnet:8.0',
          reasoning: 'Modern ASP.NET Core runtime with security updates',
          security_notes: 'Official Microsoft image with regular security patches',
          performance_notes: 'Optimized for ASP.NET Core applications'
        },
        alternatives: [
          {
            image: 'mcr.microsoft.com/dotnet/aspnet:8.0-alpine',
            use_case: 'When size optimization is critical',
            pros: ['Smaller image size', 'Faster deployment'],
            cons: ['Limited package availability', 'Potential compatibility issues']
          }
        ],
        security_considerations: {
          vulnerability_status: 'No known critical vulnerabilities',
          update_frequency: 'Monthly security updates',
          compliance: 'SOC 2, ISO 27001 compliant'
        }
      },
      'dockerfile-generation': {
        dockerfile_content: `FROM mcr.microsoft.com/dotnet/aspnet:8.0 AS base
WORKDIR /app
EXPOSE 80

FROM mcr.microsoft.com/dotnet/sdk:8.0 AS build
WORKDIR /src
COPY ["Web.csproj", "."]
RUN dotnet restore "Web.csproj"
COPY . .
RUN dotnet build "Web.csproj" -c Release -o /app/build

FROM build AS publish  
RUN dotnet publish "Web.csproj" -c Release -o /app/publish --no-restore

FROM base AS final
WORKDIR /app
COPY --from=publish /app/publish .
ENTRYPOINT ["dotnet", "Web.dll"]`,
        security_considerations: ['Non-root user', 'Minimal base image'],
        performance_optimizations: ['Multi-stage build', 'Layer caching'],
        best_practices: ['Health checks', 'Proper signal handling']
      }
    };

    const defaultResponse = defaultResponses[request.templateId];
    if (defaultResponse) {
      return {
        success: true,
        content: defaultResponse as T,
        ...ok(defaultResponse as T)
      };
    }

    return {
      success: false,
      error: { message: `No mock response for template: ${request.templateId}` },
      ...fail(`No mock response for template: ${request.templateId}`)
    };
  }

  getCallLog(): Array<{ templateId: string; variables: any }> {
    return [...this.callLog];
  }

  clearCallLog(): void {
    this.callLog = [];
  }

  clearMockResponses(): void {
    this.mockResponses.clear();
  }

  reset(): void {
    this.clearCallLog();
    this.clearMockResponses();
  }
}