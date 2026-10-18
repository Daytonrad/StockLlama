$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
$appUrl = 'http://127.0.0.1:8765'
try {
    $running = Invoke-RestMethod "$appUrl/health" -TimeoutSec 1
    if ($running.ready) { Start-Process $appUrl -WindowStyle Hidden; exit 0 }
} catch { }
$binary = Get-Item -LiteralPath 'bin\stockllama.exe' -ErrorAction SilentlyContinue
$sourceFiles = @(Get-ChildItem -LiteralPath $PSScriptRoot -Filter '*.go') + @(Get-Item 'go.mod','go.sum','ui\index.html','ui\app.js')
$needsBuild = !$binary -or @($sourceFiles | Where-Object { $_.LastWriteTimeUtc -gt $binary.LastWriteTimeUtc }).Count -gt 0
if ($needsBuild) {
    $goCommand = Get-Command go -ErrorAction SilentlyContinue
    $goPath = if ($goCommand) { $goCommand.Source } else { Join-Path $PSScriptRoot '.tools\go\bin\go.exe' }
    if (!(Test-Path -LiteralPath $goPath)) { throw 'Install Go from https://go.dev/dl/, then run start.ps1 again.' }
    New-Item -ItemType Directory -Path bin -Force | Out-Null
    $env:GOCACHE = Join-Path $PSScriptRoot '.tools\cache'
    $env:GOMODCACHE = Join-Path $PSScriptRoot '.tools\mod'
    & $goPath build -trimpath -ldflags='-s -w' -o bin\stockllama.exe .
    if ($LASTEXITCODE -ne 0) { throw 'Build failed.' }
}
$appProcess = Start-Process -FilePath (Join-Path $PSScriptRoot 'bin\stockllama.exe') -WorkingDirectory $PSScriptRoot -WindowStyle Hidden -PassThru
try {
    $ready = $false
    for ($attempt = 0; $attempt -lt 50; $attempt++) {
        if ($appProcess.HasExited) { throw 'StockLlama exited before it was ready.' }
        try { Invoke-RestMethod "$appUrl/health" -TimeoutSec 1 | Out-Null; $ready = $true; break } catch { Start-Sleep -Milliseconds 100 }
    }
    if (!$ready) { throw 'The app did not start. Check that port 8765 is free.' }
    Write-Host "StockLlama: $appUrl - press Ctrl+C to stop."
    Start-Process $appUrl -WindowStyle Hidden
    $appProcess.WaitForExit()
} finally {
    if (!$appProcess.HasExited) { Stop-Process -Id $appProcess.Id }
}
