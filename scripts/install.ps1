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

    # Read the stored value unexpanded: GetEnvironmentVariable would turn entries such as
    # %USERPROFILE%\bin into fixed paths when the new value is written back.
    $EnvironmentKey = [Microsoft.Win32.Registry]::CurrentUser.CreateSubKey("Environment")
    try {
        $UserPath = [string]$EnvironmentKey.GetValue("Path", "", [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
        $Parts = @($UserPath -split ";" | Where-Object { $_ })
        if ($Parts -notcontains $BinDir) {
            $EnvironmentKey.SetValue("Path", (($Parts + $BinDir) -join ";"), [Microsoft.Win32.RegistryValueKind]::ExpandString)
            # Changing a user variable this way tells Explorer and new terminals that the environment changed.
            [Environment]::SetEnvironmentVariable("NEXRENDER_INSTALL", "1", "User")
            [Environment]::SetEnvironmentVariable("NEXRENDER_INSTALL", $null, "User")
            $env:Path = "$env:Path;$BinDir"
        }
    } finally {
        $EnvironmentKey.Close()
    }

    Write-Host "Installed nexrender to $BinDir\nexrender.exe"
    $Nexrender = Join-Path $BinDir "nexrender.exe"
    if ($env:NEXRENDER_SKIP_SETUP -ne "1") {
        if ([Environment]::UserInteractive -and -not [Console]::IsInputRedirected) {
            & $Nexrender setup
            if ($LASTEXITCODE -ne 0) { Write-Warning "Setup did not finish. Run nexrender setup to try again." }
        } else {
            # Without a terminal, only refresh skills that the CLI installed earlier.
            # Releases before skill update existed lack the command, so check for it first.
            $Commands = @()
            try { $Commands = & $Nexrender commands --quiet --jq '.[].command' 2>$null } catch {}
            if ($Commands -contains "nexrender skill update") {
                & $Nexrender skill update
                if ($LASTEXITCODE -ne 0) { Write-Warning "Could not refresh agent skills. Run nexrender skill update to try again." }
            }
        }
    }
} finally {
    Remove-Item -Recurse -Force $TemporaryDir -ErrorAction SilentlyContinue
}
