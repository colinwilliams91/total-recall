## 1. Registry + config parity

- [x] 1.1 `internal/ai/provider.go`: add `"opencode": "https://opencode.ai/zen/go/v1"` to `ProviderRegistry` and the adapter-doc comment (`opencode → internal/ai/openai`)
- [x] 1.2 `internal/config/config.go`: add `opencode` to `KnownProviders` and its adapter comment block
- [x] 1.3 `internal/config/loader.go`: extend the user-config template's provider comment line with `opencode`
- [x] 1.4 Verify acceptance: setting `ai.provider = "opencode"`, model default `glm-5.3-flash`, and `api-key: env:OPENCODE_API_KEY` in `~/.tr/config.yaml` routes through `newProvider` to `*openai.Client` at `https://opencode.ai/zen/go/v1` (`provider_test.go`)

## 2. cmd-layer parity (`cmd/torec`)

- [x] 2.1 `main.go` `providerModelDefaults`: `"opencode": "glm-5.3-flash"`
- [x] 2.2 `main.go` `providerAPIKeyPlaceholders`: `"opencode": "env:OPENCODE_API_KEY"`
- [x] 2.3 `main.go` `runInitAI`: add `OpenCode Go` select option and include `opencode` in the cloud-provider switch case (API key + model prompts with env: pattern explanation)
- [x] 2.4 `wire.go`: add `opencode` to the unknown-provider error message known-list
- [x] 2.5 `provider_test.go`: add `opencode` to `TestNewProviderRoutesOpenAIFallback` (asserts `*openai.Client`)

## 3. Specs

- [x] 3.1 Delta spec `openspec/changes/add-opencode-provider/specs/ai-provider/spec.md` with ADDED/MODIFIED requirement for the `opencode` preset
- [x] 3.2 Sync the delta into `openspec/specs/ai-provider/spec.md` (preset list + resolution scenario)

## 4. Verification

- [x] 4.1 `go build ./... && go vet ./... && go test ./...` green
- [x] 4.2 `torec config show` resolves and displays `provider: opencode` parity (source-annotation output unchanged shape)
- [x] 4.3 Lint clean (`golangci-lint run`) where installed
