using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;

var host = Host.CreateDefaultBuilder(args)
    .ConfigureServices((context, services) =>
    {
        services.AddLogging();
    })
    .Build();

var logger = host.Services.GetRequiredService<ILogger<Program>>();

logger.LogInformation("Console application started");

// Application logic
var appService = new ApplicationService(logger);
await appService.RunAsync();

logger.LogInformation("Console application completed");

public class ApplicationService
{
    private readonly ILogger<ApplicationService> _logger;

    public ApplicationService(ILogger<ApplicationService> logger)
    {
        _logger = logger;
    }

    public async Task RunAsync()
    {
        _logger.LogInformation("Running application service...");
        
        // Simulate some work
        await Task.Delay(100);
        
        _logger.LogInformation("Application service completed");
    }
}