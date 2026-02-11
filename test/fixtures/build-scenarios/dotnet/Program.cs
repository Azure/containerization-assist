/**
 * Simple ASP.NET Core minimal API for build testing.
 * Demonstrates a minimal self-contained application.
 */

var builder = WebApplication.CreateBuilder(args);
var app = builder.Build();

app.MapGet("/", () => "Hello from .NET!");

app.MapGet("/health", () => Results.Ok("OK"));

Console.WriteLine(".NET server running on port 8080");
app.Run();
