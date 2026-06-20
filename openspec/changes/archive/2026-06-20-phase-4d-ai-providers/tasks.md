## 1. Provider Registry

- [x] 1.1 Add `qwen`, `minimax`, and `deepseek` entries to `ProviderRegistry` map in `internal/ai/provider.go` with correct base URLs

## 2. TUI Defaults

- [x] 2.1 Add `qwen`, `minimax`, and `deepseek` entries to `providerModelDefaults` map in `cmd/total-recall/main.go`
- [x] 2.2 Add `qwen`, `minimax`, and `deepseek` entries to `providerAPIKeyPlaceholders` map in `cmd/total-recall/main.go`

## 3. TUI Select Options

- [x] 3.1 Add three new `huh.NewOption` entries to the provider select form in `runInitAI()` — `Qwen (Alibaba Cloud Model Studio)`, `MiniMax (e.g. MiniMax-M3)`, `DeepSeek (e.g. deepseek-v4-pro)`
- [x] 3.2 Add `qwen`, `minimax`, `deepseek` to the cloud provider case in the `switch selectedProvider` block (alongside `anthropic`, `openai`, `groq`)

## 4. Error Messages

- [x] 4.1 Update the unknown-provider error message in `cmd/total-recall/wire.go` to include `qwen`, `minimax`, `deepseek` in the known providers list

## 5. Spec Update

- [x] 5.1 Update `openspec/specs/ai-provider/spec.md` — modify the "Named provider registry resolves base URLs" requirement to include `qwen`, `minimax`, `deepseek` in the known preset list and add scenarios for each

## 6. Documentation

- [x] 6.1 Add Phase 4D section to `ROADMAP.md` under Phase 04 — document Qwen, MiniMax, and DeepSeek provider support

## 7. Verification

- [x] 7.1 Run `go build ./...` — verify no compilation errors
- [x] 7.2 Run `go vet ./...` — verify no issues
- [x] 7.3 Run `go test ./...` — verify existing tests still pass
