$ErrorActionPreference = "Stop"

$Repository = "nexrender/nexrender-cli"
$Version = if ($env:NEXRENDER_VERSION) { $env:NEXRENDER_VERSION } else { "latest" }
$BinDir = if ($env:NEXRENDER_BIN_DIR) { $env:NEXRENDER_BIN_DIR } else { Join-Path $env:LOCALAPPDATA "Programs\nexrender" }

$Architecture = [System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture.ToString().ToLowerInvariant()
switch ($Architecture) {
    "x64" { $Arch = "amd64" }
    "arm64" { $Arch = "arm64" }
    default { throw "Unsupported architecture: $Architecture" }
}

$Archive = "nexrender_windows_${Arch}.zip"
if ($Version -eq "latest") {
    $ReleaseUrl = "https://github.com/$Repository/releases/latest/download"
} else {
    $Tag = if ($Version.StartsWith("v")) { $Version } else { "v$Version" }
    $ReleaseUrl = "https://github.com/$Repository/releases/download/$Tag"
}

$TemporaryDir = Join-Path ([System.IO.Path]::GetTempPath()) ("nexrender-install-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $TemporaryDir | Out-Null

try {
    $ArchivePath = Join-Path $TemporaryDir $Archive
    $ChecksumsPath = Join-Path $TemporaryDir "checksums.txt"
    Write-Host "Downloading $Archive from https://github.com/$Repository"
    Invoke-WebRequest -Uri "$ReleaseUrl/$Archive" -OutFile $ArchivePath
    Invoke-WebRequest -Uri "$ReleaseUrl/checksums.txt" -OutFile $ChecksumsPath

    $ArchivePattern = "\s\*?" + [regex]::Escape($Archive) + '$'
    $ChecksumLine = Get-Content $ChecksumsPath | Where-Object { $_ -match $ArchivePattern } | Select-Object -First 1
    if (-not $ChecksumLine) { throw "No checksum was published for $Archive" }
    $Expected = ($ChecksumLine -split "\s+")[0].ToLowerInvariant()
    $Actual = (Get-FileHash -Algorithm SHA256 $ArchivePath).Hash.ToLowerInvariant()
    if ($Expected -ne $Actual) { throw "Checksum verification failed" }

    Expand-Archive -Path $ArchivePath -DestinationPath $TemporaryDir -Force
    New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
    Copy-Item (Join-Path $TemporaryDir "nexrender.exe") (Join-Path $BinDir "nexrender.exe") -Force

    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $Parts = @($UserPath -split ";" | Where-Object { $_ })
    if ($Parts -notcontains $BinDir) {
        $NewPath = (($Parts + $BinDir) -join ";")
        [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
        $env:Path = "$env:Path;$BinDir"
    }

    Write-Host "Installed nexrender to $BinDir\nexrender.exe"
    if ($env:NEXRENDER_SKIP_SETUP -ne "1" -and [Environment]::UserInteractive) {
        & (Join-Path $BinDir "nexrender.exe") setup
    }
} finally {
    Remove-Item -Recurse -Force $TemporaryDir -ErrorAction SilentlyContinue
}
