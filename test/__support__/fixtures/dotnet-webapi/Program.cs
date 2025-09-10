using Microsoft.AspNetCore.Mvc;

var builder = WebApplication.CreateBuilder(args);

// Add services to the container
builder.Services.AddControllers();
builder.Services.AddEndpointsApiExplorer();
builder.Services.AddSwaggerGen();

// Add CORS
builder.Services.AddCors(options =>
{
    options.AddDefaultPolicy(policy =>
    {
        policy.AllowAnyOrigin()
              .AllowAnyHeader()
              .AllowAnyMethod();
    });
});

var app = builder.Build();

// Configure the HTTP request pipeline
if (app.Environment.IsDevelopment())
{
    app.UseSwagger();
    app.UseSwaggerUI();
}

app.UseCors();
app.UseAuthorization();
app.MapControllers();

// Health check endpoint
app.MapGet("/health", () => new
{
    Status = "Healthy",
    Timestamp = DateTime.UtcNow,
    Version = "1.0.0"
});

// Root endpoint
app.MapGet("/", () => new
{
    Message = ".NET Core Web API Test Application",
    Version = "1.0.0",
    Framework = ".NET 8.0",
    Timestamp = DateTime.UtcNow
});

// Users API endpoints
var users = new List<User>
{
    new(1, "John Doe", "john@example.com"),
    new(2, "Jane Smith", "jane@example.com")
};

app.MapGet("/api/users", () => users);

app.MapGet("/api/users/{id}", (int id) =>
{
    var user = users.FirstOrDefault(u => u.Id == id);
    return user is not null ? Results.Ok(user) : Results.NotFound();
});

app.MapPost("/api/users", ([FromBody] CreateUserRequest request) =>
{
    var user = new User(
        users.Max(u => u.Id) + 1,
        request.Name,
        request.Email
    );
    users.Add(user);
    return Results.Created($"/api/users/{user.Id}", user);
});

app.Run();

public record User(int Id, string Name, string Email);
public record CreateUserRequest(string Name, string Email);