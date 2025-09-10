using System.Text;
using Microsoft.AspNetCore.Authentication.JwtBearer;
using Microsoft.IdentityModel.Tokens;

var builder = WebApplication.CreateBuilder(args);

// Intentionally insecure JWT configuration for testing
builder.Services.AddAuthentication(JwtBearerDefaults.AuthenticationScheme)
    .AddJwtBearer(options =>
    {
        options.TokenValidationParameters = new TokenValidationParameters
        {
            ValidateIssuerSigningKey = false, // Security issue: Should be true
            IssuerSigningKey = new SymmetricSecurityKey(Encoding.UTF8.GetBytes("weak-secret-key")), // Security issue: Weak key
            ValidateIssuer = false, // Security issue: Should validate issuer
            ValidateAudience = false, // Security issue: Should validate audience
            ValidateLifetime = false, // Security issue: Should validate token lifetime
            ClockSkew = TimeSpan.Zero
        };
    });

builder.Services.AddControllers();

var app = builder.Build();

// Security issue: CORS allows all origins
app.UseCors(policy => policy.AllowAnyOrigin().AllowAnyMethod().AllowAnyHeader());

app.UseAuthentication();
app.UseAuthorization();
app.MapControllers();

app.Run();