$ErrorActionPreference = 'Stop'

$release = Invoke-RestMethod 'https://api.github.com/repos/alsolanes/gregal/releases/latest'
$assets = @($release.assets)
$extension = $assets | Where-Object { $_.name -match '^gregal-\d+\.\d+\.\d+\.vsix$' } | Select-Object -First 1
$backend = $assets | Where-Object { $_.name -eq 'gregal-windows-amd64.exe' } | Select-Object -First 1
if (-not $extension -or -not $backend) {
    throw 'La darrera publicació no inclou els paquets de VS Code i Windows. Cal una publicació recent de Gregal.'
}

$code = Get-Command code -ErrorAction SilentlyContinue
if ($code) {
    $codePath = $code.Source
} else {
    $candidates = @(
        "$env:LOCALAPPDATA\Programs\Microsoft VS Code\bin\code.cmd",
        "$env:ProgramFiles\Microsoft VS Code\bin\code.cmd"
    )
    $codePath = $candidates | Where-Object { Test-Path $_ } | Select-Object -First 1
}
if (-not $codePath) {
    throw 'No trobo VS Code ni la seva ordre code. Instal·la VS Code i torna-ho a provar.'
}

$installDir = Join-Path $env:LOCALAPPDATA 'Gregal'
New-Item -ItemType Directory -Force $installDir | Out-Null
$vsixPath = Join-Path $installDir $extension.name
$backendPath = Join-Path $installDir 'gregal.exe'
Invoke-WebRequest $extension.browser_download_url -OutFile $vsixPath
Invoke-WebRequest $backend.browser_download_url -OutFile $backendPath

& $codePath --install-extension $vsixPath --force
if ($LASTEXITCODE -ne 0) {
    throw 'VS Code no ha pogut instal·lar el paquet Gregal.'
}

Write-Host 'Gregal ja esta instal·lat. El servidor s executara en aquest terminal; deixa el terminal obert mentre facis servir VS Code.'
& $backendPath --serve
