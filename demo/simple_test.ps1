# Simple API Test Script
param(
    [string]$ServerUrl = "http://localhost:8080",
    [string]$Username = "testuser",
    [string]$Password = "test123"
)

Write-Host "Testing Figma Deliver API..." -ForegroundColor Green
Write-Host "Server: $ServerUrl" -ForegroundColor Yellow

# Test server connection
Write-Host "`n1. Testing server connection..." -ForegroundColor Cyan
try {
    $response = Invoke-WebRequest -Uri "$ServerUrl/" -Method GET -TimeoutSec 10
    Write-Host "Server is running (Status: $($response.StatusCode))" -ForegroundColor Green
} catch {
    Write-Host "Server connection failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Test user registration
Write-Host "`n2. Testing user registration..." -ForegroundColor Cyan
try {
    $registerData = "username=$Username&password=$Password&figmaToken=demo-token"
    $response = Invoke-RestMethod -Uri "$ServerUrl/register" -Method POST -Body $registerData -ContentType "application/x-www-form-urlencoded"
    Write-Host "User registration successful" -ForegroundColor Green
} catch {
    Write-Host "Registration result: User may already exist" -ForegroundColor Yellow
}

# Test plugin login
Write-Host "`n3. Testing plugin login..." -ForegroundColor Cyan
try {
    $loginData = "username=$Username&password=$Password"
    $response = Invoke-RestMethod -Uri "$ServerUrl/plugin/login" -Method POST -Body $loginData -ContentType "application/x-www-form-urlencoded"
    $token = $response.token
    Write-Host "Plugin login successful, JWT token received" -ForegroundColor Green
    Write-Host "Token preview: $($token.Substring(0, 20))..." -ForegroundColor Gray
} catch {
    Write-Host "Plugin login failed: $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host "`nAPI Test Complete!" -ForegroundColor Green
Write-Host "`nNext steps:" -ForegroundColor White
Write-Host "1. Open browser: $ServerUrl" -ForegroundColor Cyan
Write-Host "2. Login with username: $Username, password: $Password" -ForegroundColor Cyan
Write-Host "3. Install Figma plugin and configure same server URL" -ForegroundColor Cyan
