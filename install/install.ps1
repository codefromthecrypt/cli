param (
    [string]$Version,
    [string]$apexRoot = "C:\apex"
)

Write-Output ""
$ErrorActionPreference = 'stop'

#Escape space of apexRoot path
$apexRoot = $apexRoot -replace ' ', '` '

# Constants
$apexCliFileName = "apex.exe"
$apexCliFilePath = "${apexRoot}\${apexCliFileName}"
$apexCliZipExtracted = "${apexRoot}\apex_windows_amd64"

# GitHub Org and repo hosting apex CLI
$GitHubOrg = "apexlang"
$GitHubRepo = "cli"

# Set Github request authentication for basic authentication.
if ($Env:GITHUB_USER) {
    $basicAuth = [System.Convert]::ToBase64String([System.Text.Encoding]::ASCII.GetBytes($Env:GITHUB_USER + ":" + $Env:GITHUB_TOKEN));
    $githubHeader = @{"Authorization" = "Basic $basicAuth" }
}
else {
    $githubHeader = @{}
}

if ((Get-ExecutionPolicy) -gt 'RemoteSigned' -or (Get-ExecutionPolicy) -eq 'ByPass') {
    Write-Output "PowerShell requires an execution policy of 'RemoteSigned'."
    Write-Output "To make this change please run:"
    Write-Output "'Set-ExecutionPolicy RemoteSigned -scope CurrentUser'"
    break
}

# Change security protocol to support TLS 1.2 / 1.1 / 1.0 - old powershell uses TLS 1.0 as a default protocol
[Net.ServicePointManager]::SecurityProtocol = "tls12, tls11, tls"

# Check if apex CLI is installed.
if (Test-Path $apexCliFilePath -PathType Leaf) {
    Write-Warning "Apex is detected - $apexCliFilePath"
    Invoke-Expression "$apexCliFilePath version"
    Write-Output "Reinstalling Apex..."
}
else {
    Write-Output "Installing Apex..."
}

# Create apex Directory
Write-Output "Creating $apexRoot directory"
New-Item -ErrorAction Ignore -Path $apexRoot -ItemType "directory"
if (!(Test-Path $apexRoot -PathType Container)) {
    throw "Cannot create $apexRoot"
}

# Get the list of release from GitHub
$releases = Invoke-RestMethod -Headers $githubHeader -Uri "https://api.github.com/repos/${GitHubOrg}/${GitHubRepo}/releases" -Method Get
if ($releases.Count -eq 0) {
    throw "No releases from github.com/apex/cli repo"
}

# Filter windows binary and download archive
if (!$Version) {
    $windowsAsset = $releases | Where-Object { $_.tag_name -notlike "*rc*" } | Select-Object -First 1 | Select-Object -ExpandProperty assets | Where-Object { $_.name -Like "*windows_amd64.zip" }
    if (!$windowsAsset) {
        throw "Cannot find the windows apex CLI binary"
    }
    $zipFileUrl = $windowsAsset.url
    $assetName = $windowsAsset.name
} else {
    $assetName = "apex_windows_amd64.zip"
    $zipFileUrl = "https://github.com/${GitHubOrg}/${GitHubRepo}/releases/download/v${Version}/${assetName}"
}

$zipFilePath = $apexRoot + "\" + $assetName
Write-Output "Downloading $zipFileUrl ..."

$githubHeader.Accept = "application/octet-stream"
Invoke-WebRequest -Headers $githubHeader -Uri $zipFileUrl -OutFile $zipFilePath
if (!(Test-Path $zipFilePath -PathType Leaf)) {
    throw "Failed to download Apex Cli binary - $zipFilePath"
}

# Extract apex CLI to $apexRoot
Write-Output "Extracting $zipFilePath..."
Microsoft.Powershell.Archive\Expand-Archive -Force -Path $zipFilePath -DestinationPath $apexRoot
if (!(Test-Path $apexCliZipExtracted -PathType Container)) {
    throw "Failed to extract Apex Cli archieve - $apexCliZipExtracted"
}

Copy-Item $apexCliZipExtracted\${apexCliFileName} -Destination $apexRoot
Remove-Item $apexCliZipExtracted -Force -Recurse

# Clean up zipfile
Write-Output "Clean up $zipFilePath..."
Remove-Item $zipFilePath -Force

# Add apexRoot directory to User Path environment variable
Write-Output "Try to add $apexRoot to User Path Environment variable..."
$UserPathEnvironmentVar = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($UserPathEnvironmentVar -like '*apex*') {
    Write-Output "Skipping to add $apexRoot to User Path - $UserPathEnvironmentVar"
}
else {
    [System.Environment]::SetEnvironmentVariable("PATH", $UserPathEnvironmentVar + ";$apexRoot", "User")
    $UserPathEnvironmentVar = [Environment]::GetEnvironmentVariable("PATH", "User")
    Write-Output "Added $apexRoot to User Path - $UserPathEnvironmentVar"
}

# Check the apex CLI version
Invoke-Expression "$apexCliFilePath version"

Write-Output "`r`nApex CLI is installed successfully."
Write-Output "`r`nYou will need to start a new shell for the updated PATH."
