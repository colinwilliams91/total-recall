Invoke-Command -ScriptBlock {
    # One-time cleanup: previous rebuild.ps1 dropped tr.exe in the repo root.
    # The script now installs to $GOBIN so the post-commit hook resolves tr
    # via PATH, mirroring real user installs. Remove any stale root binary so
    # contributors upgrading the script don't keep a dead artifact around.
    if (Test-Path -Path "./tr.exe") {
        Write-Host "🤖🧹Removing stale ./tr.exe (rebuild.ps1 now installs to `$GOBIN)..."
        Remove-Item -Path "./tr.exe" -Force
    }

    Write-Host "🧠⚡Building and installing total-recall..."
    go install ./cmd/tr

    if ($LASTEXITCODE -ne 0) {
        Write-Host "Error installing tr. Exiting."
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
