$ErrorActionPreference = "Stop"

# Load Configuration from .aws.env
# Load Configuration from .aws.env
$EnvConfig = @{}
Get-Content .aws.env | Where-Object { $_ -match '=' } | ForEach-Object {
    $parts = $_ -split '=', 2
    if ($parts.Count -eq 2) {
        $EnvConfig[$parts[0].Trim()] = $parts[1].Trim()
    }
}

$AWS_REGION = $EnvConfig["AWS_REGION"]
$REGISTRY_URL = $EnvConfig["ECR_REGISTRY_URL"]

if (-not $AWS_REGION -or -not $REGISTRY_URL) {
    Write-Error "Missing required variables (AWS_REGION or ECR_REGISTRY_URL) in .aws.env"
    exit 1
}

$ECR_URI = "$REGISTRY_URL/platform-dashboard"
$REPO_NAME = "platform-dashboard"

Write-Host "Logging into ECR..." -ForegroundColor Green
# Use Dockerized AWS CLI to get login password and pipe to host docker login
docker run --rm --env-file .aws.env amazon/aws-cli ecr get-login-password --region $AWS_REGION | docker login --username AWS --password-stdin $REGISTRY_URL

Write-Host "Building Docker Image..." -ForegroundColor Green
docker build -t $REPO_NAME apps/platform-dashboard

Write-Host "Tagging Image..." -ForegroundColor Green
docker tag "${REPO_NAME}:latest" "${ECR_URI}:latest"

Write-Host "Pushing to ECR..." -ForegroundColor Green
docker push "${ECR_URI}:latest"

Write-Host "Deployment Artifact Pushed Successfully!" -ForegroundColor Cyan
