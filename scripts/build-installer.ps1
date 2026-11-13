param(
    [string]$ISCC = 'C:\Program Files (x86)\Inno Setup 6\ISCC.exe'
)

$ErrorActionPreference = 'Stop'
Push-Location (Split-Path $PSScriptRoot -Parent)
try {
    $config = Get-Content -Raw mygo.json | ConvertFrom-Json
    & go tool mygo build -platform windows/amd64 -skip-installer -o build/inno-app
    if ($LASTEXITCODE -ne 0) { throw 'Application build failed' }
    $source = (Resolve-Path 'build/inno-app/windows-amd64').Path
    & $ISCC "/DAppVersion=$($config.version)" "/DSourceDir=$source" 'installer/windows.iss'
    if ($LASTEXITCODE -ne 0) { throw 'Inno Setup compilation failed' }
} finally {
    Pop-Location
}
