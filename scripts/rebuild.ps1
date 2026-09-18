Invoke-Command -ScriptBlock {
    # One-time cleanup: previous rebuild.ps1 dropped tr.exe/torec.exe in the repo root.
    # The script now installs to $GOBIN so the post-commit hook resolves torec
    # via PATH, mirroring real user installs. Remove any stale root binary so
    # contributors upgrading the script don't keep a dead artifact around.
    foreach ($stale in @("./tr.exe", "./torec.exe")) {
        if (Test-Path -Path $stale) {
            Write-Host "🤖🧹Removing stale $stale (rebuild.ps1 now installs to `$GOBIN)..."
            Remove-Item -Path $stale -Force
        }
    }

    Write-Host "🧠⚡Building and installing total-recall..."
    go install ./cmd/torec

    if ($LASTEXITCODE -ne 0) {
        Write-Host "Error installing torec. Exiting."
        exit $LASTEXITCODE
    }

    Write-Host "📦⚡Building all packages..."
    go build ./...

    if ($LASTEXITCODE -ne 0) {
        Write-Host "Error building packages. Exiting."
        exit $LASTEXITCODE
    }

    Write-Host "🔍⚡Running go vet..."
    go vet ./...

    if ($LASTEXITCODE -ne 0) {
        Write-Host "Error running go vet. Exiting."
        exit $LASTEXITCODE
    }

    Write-Host "🤖💗Build completed successfully."
}
