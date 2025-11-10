export default [
  {
    "id": "dotnet-framework-48-optimal-base",
    "category": "dockerfile",
    "pattern": "FROM.*dotnet.*framework.*4\\.8",
    "recommendation": "Use .NET Framework 4.8 with Windows Server Core LTSC 2022 for optimal compatibility and security",
    "example": "FROM mcr.microsoft.com/dotnet/framework/aspnet:4.8-windowsservercore-ltsc2022 AS base\nWORKDIR /inetpub/wwwroot",
    "severity": "high",
    "tags": [
      "4.8",
      "dotnet-framework",
      "fix-dockerfile",
      "generate-dockerfile",
      "ltsc2022",
      "microsoft",
      "windowsservercore"
    ],
    "description": ".NET Framework 4.8 with LTSC 2022 provides the best balance of compatibility and security updates"
  },
  {
    "id": "aspnet-webforms-master-pages",
    "category": "dockerfile",
    "pattern": "Master|Site\\.Master|\\.master",
    "recommendation": "Configure proper file permissions and IIS settings for ASP.NET Web Forms with master pages",
    "example": "FROM mcr.microsoft.com/dotnet/framework/aspnet:4.8-windowsservercore-ltsc2022\nWORKDIR /inetpub/wwwroot\nCOPY . .\nRUN icacls . /grant \"IIS_IUSRS:(OI)(CI)RX\" /T",
    "severity": "medium",
    "tags": [
      "aspnet",
      "fix-dockerfile",
      "generate-dockerfile",
      "iis",
      "master-pages",
      "microsoft",
      "webforms"
    ],
    "description": "Web Forms applications require proper file permissions for master pages and user controls"
  },
  {
    "id": "aspnet-webforms-viewstate",
    "category": "security",
    "pattern": "ViewState|__VIEWSTATE",
    "recommendation": "Secure ViewState with machine keys and consider disabling for stateless containers",
    "example": "<!-- Web.config -->\n<system.web>\n  <pages enableViewStateMac=\"true\" viewStateEncryptionMode=\"Always\" />\n  <machineKey validationKey=\"[64-hex-chars]\" decryptionKey=\"[48-hex-chars]\" />\n</system.web>",
    "severity": "high",
    "tags": [
      "aspnet",
      "aws",
      "fix-dockerfile",
      "machinekey",
      "scan-image",
      "security",
      "viewstate",
      "webforms"
    ],
    "description": "ViewState should be properly secured with consistent machine keys across container instances"
  },
  {
    "id": "aspnet-session-state-containerization",
    "category": "dockerfile",
    "pattern": "Session|SessionState",
    "recommendation": "Use external session storage (SQL Server, Redis) for scalable container deployments",
    "example": "<!-- Web.config -->\n<system.web>\n  <sessionState mode=\"SQLServer\" \n                cookieless=\"false\" \n                connectionString=\"server=sql;database=sessions;\" \n                timeout=\"20\" />\n</system.web>",
    "severity": "high",
    "tags": [
      "aspnet",
      "fix-dockerfile",
      "generate-dockerfile",
      "redis",
      "scalability",
      "session",
      "sqlserver"
    ],
    "description": "In-memory session state doesn't work with multiple container instances"
  },
  {
    "id": "aspnet-application-warm-up",
    "category": "dockerfile",
    "pattern": "Application_Start|Global\\.asax",
    "recommendation": "Implement application warm-up to reduce first request latency in containers",
    "example": "FROM mcr.microsoft.com/dotnet/framework/aspnet:4.8-windowsservercore-ltsc2022\nWORKDIR /inetpub/wwwroot\nCOPY . .\n# Warm-up application\nRUN powershell -Command \"Start-Process w3wp -ArgumentList '-debug' -PassThru | Out-Null; Invoke-WebRequest -Uri http://localhost -UseBasicParsing\"",
    "severity": "medium",
    "tags": [
      "aspnet",
      "fix-dockerfile",
      "generate-dockerfile",
      "global-asax",
      "microsoft",
      "performance",
      "warmup"
    ],
    "description": "Application warm-up reduces cold start latency for containerized ASP.NET applications"
  },
  {
    "id": "wcf-service-windows-containers",
    "category": "dockerfile",
    "pattern": "\\.svc|ServiceModel|WCF",
    "recommendation": "WCF services require full Windows Server Core containers with proper endpoint configuration",
    "example": "FROM mcr.microsoft.com/dotnet/framework/wcf:4.8-windowsservercore-ltsc2022\nWORKDIR /inetpub/wwwroot\nCOPY . .\nEXPOSE 80 808\n# Configure WCF service endpoints in Web.config",
    "severity": "high",
    "tags": [
      "endpoints",
      "fix-dockerfile",
      "generate-dockerfile",
      "microsoft",
      "servicemodel",
      "wcf",
      "windows-containers"
    ],
    "description": "WCF services need Windows containers and cannot run on Linux containers"
  },
  {
    "id": "entity-framework-6-connectionstrings",
    "category": "dockerfile",
    "pattern": "EntityFramework|EF6|DbContext.*6\\.",
    "recommendation": "Use environment variables for Entity Framework 6 connection strings in containers",
    "example": "ENV CONNECTION_STRING=\"Data Source=sqlserver;Initial Catalog=MyDb;Integrated Security=False;User ID=sa;Password=${SA_PASSWORD}\"\n# In App.config transformation\n<connectionStrings>\n  <add name=\"DefaultConnection\" connectionString=\"%CONNECTION_STRING%\" />\n</connectionStrings>",
    "severity": "medium",
    "tags": [
      "connectionstring",
      "ef6",
      "entityframework",
      "environment",
      "fix-dockerfile",
      "generate-dockerfile"
    ],
    "description": "Entity Framework 6 connection strings should be externalized for container environments"
  },
  {
    "id": "windows-service-container-hosting",
    "category": "dockerfile",
    "pattern": "ServiceBase|System\\.ServiceProcess",
    "recommendation": "Convert Windows Services to console applications or use Windows Server Core containers",
    "example": "FROM mcr.microsoft.com/dotnet/framework/runtime:4.8-windowsservercore-ltsc2022\nWORKDIR /app\nCOPY . .\n# Run as console app instead of Windows Service\nCMD [\"MyService.exe\", \"/console\"]",
    "severity": "medium",
    "tags": [
      "console",
      "fix-dockerfile",
      "generate-dockerfile",
      "hosting",
      "microsoft",
      "servicebase",
      "windows-service"
    ],
    "description": "Windows Services need modification to run effectively in containers"
  },
  {
    "id": "iis-application-pool-optimization",
    "category": "dockerfile",
    "pattern": "ApplicationPool|AppPool",
    "recommendation": "Optimize IIS application pools for container resource constraints",
    "example": "RUN powershell -Command \"\\\n  Import-Module WebAdministration; \\\n  Set-ItemProperty 'IIS:\\\\AppPools\\\\DefaultAppPool' -Name processModel.idleTimeout -Value 00:00:00; \\\n  Set-ItemProperty 'IIS:\\\\AppPools\\\\DefaultAppPool' -Name recycling.periodicRestart.time -Value 00:00:00; \\\n  Set-ItemProperty 'IIS:\\\\AppPools\\\\DefaultAppPool' -Name processModel.maxProcesses -Value 1\"",
    "severity": "medium",
    "tags": [
      "application-pool",
      "fix-dockerfile",
      "generate-dockerfile",
      "iis",
      "optimization",
      "resources"
    ],
    "description": "IIS application pools should be configured for container lifecycle and resource limits"
  },
  {
    "id": "dotnet-framework-temp-files",
    "category": "dockerfile",
    "pattern": "Temporary ASP\\.NET Files|temp",
    "recommendation": "Configure proper permissions and location for ASP.NET temporary files",
    "example": "RUN icacls \"C:\\\\Windows\\\\Microsoft.NET\\\\Framework64\\\\v4.0.30319\\\\Temporary ASP.NET Files\" /grant \"IIS_IUSRS:(OI)(CI)F\" /T\nRUN icacls \"C:\\\\Windows\\\\Temp\" /grant \"IIS_IUSRS:(OI)(CI)F\" /T",
    "severity": "medium",
    "tags": [
      "aspnet",
      "compilation",
      "fix-dockerfile",
      "generate-dockerfile",
      "microsoft",
      "permissions",
      "temporary-files"
    ],
    "description": "ASP.NET requires write access to temporary compilation directories"
  },
  {
    "id": "aspnet-compilation-debug-retail",
    "category": "dockerfile",
    "pattern": "debug=\"true\"|compilation.*debug",
    "recommendation": "Always use retail compilation mode in production containers",
    "example": "<!-- Web.config for production -->\n<system.web>\n  <compilation debug=\"false\" targetFramework=\"4.8\" tempDirectory=\"C:\\\\temp\\\\aspnet\" />\n  <httpRuntime targetFramework=\"4.8\" enableVersionHeader=\"false\" />\n</system.web>",
    "severity": "high",
    "tags": [
      "aspnet",
      "compilation",
      "debug",
      "fix-dockerfile",
      "generate-dockerfile",
      "performance",
      "production"
    ],
    "description": "Debug mode should never be enabled in production containers"
  },
  {
    "id": "crystal-reports-container-limitations",
    "category": "dockerfile",
    "pattern": "CrystalDecisions|Crystal.*Report",
    "recommendation": "Crystal Reports has limited container support, consider alternatives or use Windows Server Core",
    "example": "# Crystal Reports requires full Windows containers\nFROM mcr.microsoft.com/dotnet/framework/aspnet:4.8-windowsservercore-ltsc2022\n# Install Crystal Reports runtime\n# Consider migrating to SSRS, Power BI, or other reporting solutions",
    "severity": "low",
    "tags": [
      "alternatives",
      "compatibility",
      "crystal-reports",
      "fix-dockerfile",
      "generate-dockerfile",
      "microsoft",
      "reporting"
    ],
    "description": "Crystal Reports has compatibility issues in containers; consider modern alternatives"
  },
  {
    "id": "aspnet-health-monitoring",
    "category": "dockerfile",
    "pattern": "healthMonitoring|WebEvent",
    "recommendation": "Replace ASP.NET health monitoring with container-native health checks",
    "example": "HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \\\n  CMD powershell -Command \"try { \\\n    $response = Invoke-WebRequest -Uri http://localhost/health.aspx -UseBasicParsing -TimeoutSec 10; \\\n    if ($response.StatusCode -eq 200) { exit 0 } else { exit 1 } \\\n  } catch { exit 1 }\"",
    "severity": "medium",
    "tags": [
      "aspnet",
      "fix-dockerfile",
      "generate-dockerfile",
      "health-monitoring",
      "healthcheck",
      "monitoring"
    ],
    "description": "Use Docker health checks instead of ASP.NET health monitoring in containers"
  },
  {
    "id": "dotnet-framework-gac-alternatives",
    "category": "dockerfile",
    "pattern": "gacutil|Global Assembly Cache",
    "recommendation": "Avoid GAC in containers; use private assemblies in bin directory",
    "example": "# Instead of GAC installation:\n# RUN gacutil /i MySharedLibrary.dll\n# Use private assemblies:\nCOPY MySharedLibrary.dll bin/\nCOPY OtherDependencies/*.dll bin/",
    "severity": "high",
    "tags": [
      "assemblies",
      "deployment",
      "fix-dockerfile",
      "gac",
      "generate-dockerfile",
      "private-assemblies"
    ],
    "description": "GAC complicates container deployment; use private assemblies for better portability"
  },
  {
    "id": "aspnet-profile-provider-external",
    "category": "dockerfile",
    "pattern": "Profile|ProfileProvider",
    "recommendation": "Use external profile storage (SQL Server, Azure) instead of local providers",
    "example": "<!-- Web.config -->\n<system.web>\n  <profile enabled=\"true\" defaultProvider=\"SqlProvider\">\n    <providers>\n      <add name=\"SqlProvider\" \n           type=\"System.Web.Profile.SqlProfileProvider\" \n           connectionStringName=\"ProfileConnection\" />\n    </providers>\n  </profile>\n</system.web>",
    "severity": "medium",
    "tags": [
      "aspnet",
      "azure",
      "external-storage",
      "fix-dockerfile",
      "generate-dockerfile",
      "profile",
      "sql-provider"
    ],
    "description": "Profile data should be stored externally for scalable container deployments"
  },
  {
    "id": "aspnet-membership-provider-sql",
    "category": "dockerfile",
    "pattern": "Membership|MembershipProvider",
    "recommendation": "Use SQL Server membership provider for stateless container deployments",
    "example": "<!-- Web.config -->\n<system.web>\n  <membership defaultProvider=\"SqlMembershipProvider\">\n    <providers>\n      <add name=\"SqlMembershipProvider\" \n           type=\"System.Web.Security.SqlMembershipProvider\" \n           connectionStringName=\"MembershipConnection\" \n           requiresUniqueEmail=\"true\" />\n    </providers>\n  </membership>\n</system.web>",
    "severity": "medium",
    "tags": [
      "aspnet",
      "authentication",
      "fix-dockerfile",
      "generate-dockerfile",
      "membership",
      "sql-provider"
    ],
    "description": "Membership data should be externalized for container scalability"
  },
  {
    "id": "dotnet-framework-x64-containers",
    "category": "dockerfile",
    "pattern": "AnyCPU|x86|x64",
    "recommendation": "Use AnyCPU or x64 builds for optimal container performance on modern hosts",
    "example": "# Build configuration for containers\nRUN msbuild MyProject.sln /p:Configuration=Release /p:Platform=\"Any CPU\" /p:Prefer32Bit=false",
    "severity": "low",
    "tags": [
      "anycpu",
      "architecture",
      "fix-dockerfile",
      "generate-dockerfile",
      "performance",
      "x64"
    ],
    "description": "64-bit builds perform better in modern container environments"
  },
  {
    "id": "aspnet-webforms-updatepanel",
    "category": "dockerfile",
    "pattern": "UpdatePanel|ScriptManager",
    "recommendation": "AJAX UpdatePanels work in containers but may need script path configuration",
    "example": "<!-- Web.config -->\n<system.web>\n  <httpHandlers>\n    <add path=\"ScriptResource.axd\" type=\"System.Web.Handlers.ScriptResourceHandler\" validate=\"false\" />\n  </httpHandlers>\n</system.web>",
    "severity": "low",
    "tags": [
      "ajax",
      "aspnet",
      "fix-dockerfile",
      "generate-dockerfile",
      "scriptmanager",
      "updatepanel"
    ],
    "description": "AJAX features need proper handler configuration in containerized environments"
  },
  {
    "id": "dotnet-framework-logging-structured",
    "category": "dockerfile",
    "pattern": "log4net|NLog|System\\.Diagnostics\\.Trace",
    "recommendation": "Configure structured logging to stdout for container log aggregation",
    "example": "<!-- Configure log4net for containers -->\n<log4net>\n  <appender name=\"ConsoleAppender\" type=\"log4net.Appender.ConsoleAppender\">\n    <layout type=\"log4net.Layout.JsonLayout\" />\n  </appender>\n  <root>\n    <level value=\"INFO\" />\n    <appender-ref ref=\"ConsoleAppender\" />\n  </root>\n</log4net>",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "log4net",
      "logging",
      "nlog",
      "stdout",
      "structured"
    ],
    "description": "Container logs should be structured and sent to stdout for proper aggregation"
  },
  {
    "id": "aspnet-custom-errors-container",
    "category": "security",
    "pattern": "customErrors|Custom.*Error",
    "recommendation": "Configure custom errors for production containers to avoid information disclosure",
    "example": "<!-- Web.config -->\n<system.web>\n  <customErrors mode=\"On\" defaultRedirect=\"~/Error.aspx\">\n    <error statusCode=\"404\" redirect=\"~/NotFound.aspx\" />\n    <error statusCode=\"500\" redirect=\"~/ServerError.aspx\" />\n  </customErrors>\n  <httpRuntime enableVersionHeader=\"false\" />\n</system.web>",
    "severity": "high",
    "tags": [
      "aspnet",
      "custom-errors",
      "fix-dockerfile",
      "production",
      "scan-image",
      "security"
    ],
    "description": "Custom errors should be properly configured to avoid exposing sensitive information"
  },
  {
    "id": "dotnet-framework-nuget-restore-cache",
    "category": "dockerfile",
    "pattern": "packages\\.config|PackageReference.*4\\.",
    "recommendation": "Restore NuGet packages in separate layer for better Docker build caching",
    "example": "FROM mcr.microsoft.com/dotnet/framework/sdk:4.8-windowsservercore-ltsc2022 AS build\nWORKDIR /src\n# Copy and restore packages first\nCOPY packages.config .\nCOPY *.sln .\nRUN nuget restore\n# Then copy source and build\nCOPY . .\nRUN msbuild /p:Configuration=Release",
    "severity": "medium",
    "tags": [
      "build-optimization",
      "caching",
      "fix-dockerfile",
      "generate-dockerfile",
      "microsoft",
      "nuget",
      "packages"
    ],
    "description": "Package restoration should be cached separately from source code changes"
  },
  {
    "id": "aspnet-webforms-themes-skins",
    "category": "dockerfile",
    "pattern": "Theme|SkinID|\\.skin",
    "recommendation": "Ensure theme files have proper permissions and are included in container build",
    "example": "FROM mcr.microsoft.com/dotnet/framework/aspnet:4.8-windowsservercore-ltsc2022\nWORKDIR /inetpub/wwwroot\nCOPY . .\n# Ensure theme directories are accessible\nRUN icacls App_Themes /grant \"IIS_IUSRS:(OI)(CI)RX\" /T",
    "severity": "low",
    "tags": [
      "aspnet",
      "fix-dockerfile",
      "generate-dockerfile",
      "microsoft",
      "permissions",
      "skins",
      "themes",
      "webforms"
    ],
    "description": "Theme and skin files need proper permissions in containerized Web Forms applications"
  },
  {
    "id": "dotnet-framework-startup-performance",
    "category": "dockerfile",
    "pattern": "Application_Start|PreApplicationStartMethod",
    "recommendation": "Optimize application startup performance for faster container scaling",
    "example": "<!-- Web.config optimizations -->\n<system.web>\n  <compilation debug=\"false\" strict=\"false\" explicit=\"true\" targetFramework=\"4.8\" optimizeCompilations=\"true\" />\n  <httpRuntime targetFramework=\"4.8\" relaxedUrlToFileSystemMapping=\"true\" />\n  <trust level=\"Full\" />\n</system.web>",
    "severity": "medium",
    "tags": [
      "compilation",
      "fix-dockerfile",
      "generate-dockerfile",
      "optimization",
      "performance",
      "startup"
    ],
    "description": "Application startup should be optimized for faster container instance scaling"
  },
  {
    "id": "aspnet-localization-containers",
    "category": "dockerfile",
    "pattern": "Localization|GlobalResources|LocalResources",
    "recommendation": "Include all resource files and configure proper culture settings for containers",
    "example": "FROM mcr.microsoft.com/dotnet/framework/aspnet:4.8-windowsservercore-ltsc2022\nWORKDIR /inetpub/wwwroot\nCOPY . .\n# Set default culture\nENV ASPNET_CULTURE=\"en-US\"\nENV ASPNET_UICULTURE=\"en-US\"",
    "severity": "low",
    "tags": [
      "aspnet",
      "culture",
      "fix-dockerfile",
      "generate-dockerfile",
      "localization",
      "microsoft",
      "resources"
    ],
    "description": "Localization resources and culture settings should be properly configured for containers"
  },
  {
    "id": "wcf-service-behaviors-container",
    "category": "dockerfile",
    "pattern": "serviceBehaviors|Service.*Behavior",
    "recommendation": "Configure WCF service behaviors for container environments and external dependencies",
    "example": "<!-- Web.config -->\n<system.serviceModel>\n  <behaviors>\n    <serviceBehaviors>\n      <behavior name=\"MyServiceBehavior\">\n        <serviceMetadata httpGetEnabled=\"true\" />\n        <serviceDebug includeExceptionDetailInFaults=\"false\" />\n        <serviceThrottling maxConcurrentCalls=\"100\" maxConcurrentSessions=\"100\" />\n      </behavior>\n    </serviceBehaviors>\n  </behaviors>\n</system.serviceModel>",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "generate-dockerfile",
      "metadata",
      "service-behaviors",
      "throttling",
      "wcf"
    ],
    "description": "WCF service behaviors should be optimized for container resource constraints"
  },
  {
    "id": "dotnet-framework-memory-optimization",
    "category": "dockerfile",
    "pattern": "OutOfMemoryException|Memory",
    "recommendation": "Configure memory settings and GC options for .NET Framework containers",
    "example": "<!-- Web.config -->\n<system.web>\n  <httpRuntime maxRequestLength=\"51200\" executionTimeout=\"3600\" targetFramework=\"4.8\" />\n  <processModel maxWorkerThreads=\"100\" maxIoThreads=\"100\" memoryLimit=\"80\" />\n</system.web>\n# Environment variables\nENV DOTNET_GCServer=1\nENV DOTNET_gcAllowVeryLargeObjects=1",
    "severity": "medium",
    "tags": [
      "fix-dockerfile",
      "gc",
      "generate-dockerfile",
      "memory",
      "optimization",
      "performance"
    ],
    "description": "Memory and GC settings should be tuned for container resource limits"
  },
  {
    "id": "aspnet-output-caching-redis",
    "category": "dockerfile",
    "pattern": "OutputCache|VaryByParam",
    "recommendation": "Use Redis or external cache providers for output caching in scalable containers",
    "example": "<!-- Web.config -->\n<system.web>\n  <caching>\n    <outputCacheSettings>\n      <outputCacheProfiles>\n        <add name=\"CacheProfile\" enabled=\"true\" duration=\"3600\" varyByParam=\"*\" />\n      </outputCacheProfiles>\n    </outputCacheSettings>\n  </caching>\n</system.web>\n# Consider Redis output cache provider",
    "severity": "medium",
    "tags": [
      "aspnet",
      "fix-dockerfile",
      "generate-dockerfile",
      "output-cache",
      "redis",
      "scalability"
    ],
    "description": "Output caching should use external providers for multi-container deployments"
  },
  {
    "id": "dotnet-framework-config-transforms",
    "category": "dockerfile",
    "pattern": "Web\\.config|App\\.config",
    "recommendation": "Use configuration transforms and environment variables for container deployments",
    "example": "# Build stage with config transformation\nRUN msbuild /p:Configuration=Release /p:PublishProfile=Docker /p:WebPublishMethod=FileSystem\n# Runtime stage\nFROM mcr.microsoft.com/dotnet/framework/aspnet:4.8-windowsservercore-ltsc2022\nWORKDIR /inetpub/wwwroot\nCOPY --from=build /app/publish .\nENV CONNECTION_STRING=\"Server=db;Database=prod;\"",
    "severity": "medium",
    "tags": [
      "configuration",
      "deployment",
      "environment",
      "fix-dockerfile",
      "generate-dockerfile",
      "microsoft",
      "transforms"
    ],
    "description": "Configuration should be transformed and externalized for different container environments"
  },
  {
    "id": "aspnet-webforms-validation-groups",
    "category": "dockerfile",
    "pattern": "ValidationGroup|RequiredFieldValidator",
    "recommendation": "Web Forms validation works in containers but ensure client-side scripts are properly configured",
    "example": "<!-- Ensure validation scripts are accessible -->\n<system.web>\n  <httpHandlers>\n    <add path=\"WebResource.axd\" type=\"System.Web.Handlers.AssemblyResourceLoader\" validate=\"false\" />\n  </httpHandlers>\n  <pages enableViewState=\"true\" validateRequest=\"true\" />\n</system.web>",
    "severity": "low",
    "tags": [
      "aspnet",
      "client-scripts",
      "fix-dockerfile",
      "generate-dockerfile",
      "validation",
      "webforms"
    ],
    "description": "Web Forms validation scripts need proper handler configuration in containers"
  },
  {
    "id": "dotnet-framework-security-headers",
    "category": "security",
    "pattern": "httpHeaders|Security.*Header",
    "recommendation": "Configure security headers for ASP.NET Framework applications in containers",
    "example": "<!-- Web.config -->\n<system.webServer>\n  <httpProtocol>\n    <customHeaders>\n      <add name=\"X-Frame-Options\" value=\"DENY\" />\n      <add name=\"X-Content-Type-Options\" value=\"nosniff\" />\n      <add name=\"X-XSS-Protection\" value=\"1; mode=block\" />\n      <add name=\"Strict-Transport-Security\" value=\"max-age=31536000; includeSubDomains\" />\n    </customHeaders>\n  </httpProtocol>\n</system.webServer>",
    "severity": "high",
    "tags": [
      "clickjacking",
      "fix-dockerfile",
      "headers",
      "scan-image",
      "security",
      "xss"
    ],
    "description": "Security headers should be configured to protect against common web vulnerabilities"
  },
  {
    "id": "dotnet-framework-deployment-verification",
    "category": "dockerfile",
    "pattern": "Assembly.*Load|Assembly.*Resolve",
    "recommendation": "Verify all assemblies are properly deployed and accessible in container environment",
    "example": "# Verify deployment in build stage\nRUN powershell -Command \"Get-ChildItem -Path . -Recurse -Include *.dll | ForEach-Object { \\\n  try { [System.Reflection.Assembly]::LoadFrom($_.FullName) | Out-Null; Write-Host \\\"OK: $($_.Name)\\\" } \\\n  catch { Write-Error \\\"Failed to load: $($_.Name)\\\" } }\"",
    "severity": "low",
    "tags": [
      "assemblies",
      "dependencies",
      "deployment",
      "fix-dockerfile",
      "generate-dockerfile",
      "verification"
    ],
    "description": "Assembly loading should be verified during container build to catch deployment issues early"
  }
] as const;
