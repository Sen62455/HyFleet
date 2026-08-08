[CmdletBinding()]
param(
    [ValidateSet("amd64", "arm64")]
    [string]$Architecture = "amd64",

    [ValidatePattern("^[0-9A-Za-z][0-9A-Za-z._-]*$")]
    [string]$Version = "v0.5.0-dev"
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repositoryRoot = Split-Path -Parent $PSScriptRoot
$releaseRoot = Join-Path (Join-Path $repositoryRoot "output") "releases"
$packageName = "hyfleet-$Version-linux-$Architecture"
$bundlePath = Join-Path $releaseRoot $packageName
$archivePath = Join-Path $releaseRoot "$packageName.tar.gz"
$archiveChecksumPath = "$archivePath.sha256"

function Invoke-Checked {
    param(
        [Parameter(Mandatory)]
        [string]$Command,

        [Parameter(ValueFromRemainingArguments)]
        [string[]]$Arguments
    )

    & $Command @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$Command exited with code $LASTEXITCODE"
    }
}

foreach ($commandName in @("git", "go", "pnpm", "tar")) {
    if (-not (Get-Command $commandName -ErrorAction SilentlyContinue)) {
        throw "Required command is missing: $commandName"
    }
}

New-Item -ItemType Directory -Force -Path $releaseRoot | Out-Null
if (Test-Path -LiteralPath $bundlePath) {
    Remove-Item -LiteralPath $bundlePath -Recurse -Force
}
if (Test-Path -LiteralPath $archivePath) {
    Remove-Item -LiteralPath $archivePath -Force
}
if (Test-Path -LiteralPath $archiveChecksumPath) {
    Remove-Item -LiteralPath $archiveChecksumPath -Force
}

$commit = "uncommitted"
$resolvedCommit = & git -C $repositoryRoot rev-parse --short HEAD 2>$null
if ($LASTEXITCODE -eq 0 -and $resolvedCommit) {
    $commit = $resolvedCommit.Trim()
}
$buildDate = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")
$ldflags = "-s -w " +
    "-X github.com/hyfleet/hyfleet/internal/buildinfo.Version=$Version " +
    "-X github.com/hyfleet/hyfleet/internal/buildinfo.Commit=$commit " +
    "-X github.com/hyfleet/hyfleet/internal/buildinfo.Date=$buildDate"

Push-Location (Join-Path $repositoryRoot "web")
try {
    Invoke-Checked pnpm install --frozen-lockfile
    Invoke-Checked pnpm build
}
finally {
    Pop-Location
}

$binaryOutput = Join-Path $releaseRoot ".build-$Architecture"
New-Item -ItemType Directory -Force -Path $binaryOutput | Out-Null
$previousCgo = $env:CGO_ENABLED
$previousGoos = $env:GOOS
$previousGoarch = $env:GOARCH
try {
    $env:CGO_ENABLED = "0"
    $env:GOOS = "linux"
    $env:GOARCH = $Architecture
    Push-Location $repositoryRoot
    try {
        $serverBuildArguments = @(
            "build",
            "-tags=webui",
            "-ldflags=$ldflags",
            "-o=$(Join-Path $binaryOutput 'hyfleet-server')",
            "./cmd/server"
        )
        Invoke-Checked -Command go -Arguments $serverBuildArguments

        $agentBuildArguments = @(
            "build",
            "-ldflags=$ldflags",
            "-o=$(Join-Path $binaryOutput 'hyfleet-agent')",
            "./cmd/agent"
        )
        Invoke-Checked -Command go -Arguments $agentBuildArguments
    }
    finally {
        Pop-Location
    }
}
finally {
    $env:CGO_ENABLED = $previousCgo
    $env:GOOS = $previousGoos
    $env:GOARCH = $previousGoarch
}

$expectedMachine = if ($Architecture -eq "amd64") { 0x3e } else { 0xb7 }
foreach ($binaryName in @("hyfleet-server", "hyfleet-agent")) {
    $binaryPath = Join-Path $binaryOutput $binaryName
    $stream = [System.IO.File]::OpenRead($binaryPath)
    try {
        $header = New-Object byte[] 20
        if ($stream.Read($header, 0, $header.Length) -ne $header.Length) {
            throw "$binaryName is too short to be an ELF binary"
        }
    }
    finally {
        $stream.Dispose()
    }
    if ($header[0] -ne 0x7f -or $header[1] -ne 0x45 -or
        $header[2] -ne 0x4c -or $header[3] -ne 0x46) {
        throw "$binaryName is not an ELF binary"
    }
    $machine = $header[18] -bor ($header[19] -shl 8)
    if ($machine -ne $expectedMachine) {
        throw "$binaryName has unexpected ELF architecture $machine"
    }
}

foreach ($directory in @("bin", "configs", (Join-Path "deploy" "systemd"), "docs")) {
    New-Item -ItemType Directory -Force -Path (Join-Path $bundlePath $directory) | Out-Null
}
Copy-Item (Join-Path $binaryOutput "hyfleet-server") (Join-Path $bundlePath "bin")
Copy-Item (Join-Path $binaryOutput "hyfleet-agent") (Join-Path $bundlePath "bin")
$configSource = Join-Path (Join-Path $repositoryRoot "configs") "*"
$unitSource = Join-Path (Join-Path (Join-Path $repositoryRoot "deploy") "systemd") "*"
Copy-Item $configSource (Join-Path $bundlePath "configs")
Copy-Item $unitSource (Join-Path (Join-Path $bundlePath "deploy") "systemd")
foreach ($scriptName in @("install-server.sh", "install-agent.sh", "diagnose.sh", "configure-hysteria.sh", "update-component.sh")) {
    Copy-Item (Join-Path (Join-Path $repositoryRoot "deploy") $scriptName) (Join-Path $bundlePath "deploy")
}
foreach ($documentName in @("10-systemd-deployment.md", "11-phase-2-native-users.md", "12-phase-3-traffic-and-updates.md", "13-phase-4-unified-subscriptions.md", "14-phase-5-sui-adapter.md")) {
    Copy-Item (Join-Path (Join-Path $repositoryRoot "docs") $documentName) (Join-Path $bundlePath "docs")
}

$textFiles = Get-ChildItem -Path $bundlePath -Recurse -File | Where-Object {
    $_.Extension -in @(".sh", ".service", ".yaml", ".example", ".md")
}
foreach ($textFile in $textFiles) {
    $content = [System.IO.File]::ReadAllText($textFile.FullName).Replace("`r`n", "`n")
    [System.IO.File]::WriteAllText(
        $textFile.FullName,
        $content,
        [System.Text.UTF8Encoding]::new($false)
    )
}

[System.IO.File]::WriteAllText(
    (Join-Path $bundlePath "VERSION"),
    "$Version`n",
    [System.Text.UTF8Encoding]::new($false)
)

$bundleRootPath = [System.IO.Path]::GetFullPath($bundlePath)
if (-not $bundleRootPath.EndsWith([System.IO.Path]::DirectorySeparatorChar)) {
    $bundleRootPath += [System.IO.Path]::DirectorySeparatorChar
}

$checksumLines = Get-ChildItem -Path $bundlePath -Recurse -File |
    Sort-Object FullName |
    ForEach-Object {
        $fullPath = [System.IO.Path]::GetFullPath($_.FullName)
        if (-not $fullPath.StartsWith($bundleRootPath, [System.StringComparison]::OrdinalIgnoreCase)) {
            throw "Package file is outside the bundle root: $fullPath"
        }
        $relativePath = $fullPath.Substring($bundleRootPath.Length).Replace("\", "/")
        $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $_.FullName).Hash.ToLowerInvariant()
        "$hash  $relativePath"
    }
[System.IO.File]::WriteAllLines(
    (Join-Path $bundlePath "SHA256SUMS"),
    $checksumLines,
    [System.Text.UTF8Encoding]::new($false)
)

$tarArguments = @("-czf", $archivePath, "-C", $releaseRoot, $packageName)
Invoke-Checked -Command tar -Arguments $tarArguments
$archiveHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $archivePath).Hash.ToLowerInvariant()
[System.IO.File]::WriteAllText(
    $archiveChecksumPath,
    "$archiveHash  $([System.IO.Path]::GetFileName($archivePath))`n",
    [System.Text.UTF8Encoding]::new($false)
)

Write-Host "Created: $archivePath"
Write-Host "SHA256: $archiveHash"
