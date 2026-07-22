Invoke-Command -ScriptBlock {
    if (Test-Path -Path "./tr.exe") {
        Write-Host "🤖🧹Removing existing tr.exe for a clean build..."
        Remove-Item -Path "./tr.exe" -Force
    }

    Write-Host "🧠⚡Building tr..."
    go build -o tr.exe ./cmd/tr

    if ($LASTEXITCODE -ne 0) {
        Write-Host "Error building tr. Exiting."
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
