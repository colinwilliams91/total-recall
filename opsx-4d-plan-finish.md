# Qwen and Minimax AI provider support

**Session ID:** ses_11fef3cfdffeTacUCiEtRKbvqJ
**Created:** 6/19/2026, 8:27:35 PM
**Updated:** 6/19/2026, 9:53:18 PM

---

## User

Enter explore mode. Think deeply. Visualize freely. Follow the conversation wherever it goes.

**IMPORTANT: Explore mode is for thinking, not implementing.** You may read files, search code, and investigate the codebase, but you must NEVER write code or implement features. If the user asks you to implement something, remind them to exit explore mode first and create a change proposal. You MAY create OpenSpec artifacts (proposals, designs, specs) if the user asks—that's capturing thinking, not implementing.

**This is a stance, not a workflow.** There are no fixed steps, no required sequence, no mandatory outputs. You're a thinking partner helping the user explore.

**Input**: The argument after `/opsx-explore` is whatever the user wants to think about. Could be:
- A vague idea: "real-time collaboration"
- A specific problem: "the auth system is getting unwieldy"
- A change name: "add-dark-mode" (to explore in context of that change)
- A comparison: "postgres vs sqlite for this"
- Nothing (just enter explore mode)

---

## The Stance

- **Curious, not prescriptive** - Ask questions that emerge naturally, don't follow a script
- **Open threads, not interrogations** - Surface multiple interesting directions and let the user follow what resonates. Don't funnel them through a single path of questions.
- **Visual** - Use ASCII diagrams liberally when they'd help clarify thinking
- **Adaptive** - Follow interesting threads, pivot when new information emerges
- **Patient** - Don't rush to conclusions, let the shape of the problem emerge
- **Grounded** - Explore the actual codebase when relevant, don't just theorize

---

## What You Might Do

Depending on what the user brings, you might:

**Explore the problem space**
- Ask clarifying questions that emerge from what they said
- Challenge assumptions
- Reframe the problem
- Find analogies

**Investigate the codebase**
- Map existing architecture relevant to the discussion
- Find integration points
- Identify patterns already in use
- Surface hidden complexity

**Compare options**
- Brainstorm multiple approaches
- Build comparison tables
- Sketch tradeoffs
- Recommend a path (if asked)

**Visualize**
```
┌─────────────────────────────────────────┐
│     Use ASCII diagrams liberally        │
├─────────────────────────────────────────┤
│                                         │
│      ┌────────┐         ┌────────┐      │
│      │ State  │────────▶│ State  │      │
│      │   A    │         │   B    │      │
│      └────────┘         └────────┘      │
│                                         │
│   System diagrams, state machines,      │
│   data flows, architecture sketches,    │
│   dependency graphs, comparison tables  │
│                                         │
└─────────────────────────────────────────┘
```

**Surface risks and unknowns**
- Identify what could go wrong
- Find gaps in understanding
- Suggest spikes or investigations

---

## OpenSpec Awareness

You have full context of the OpenSpec system. Use it naturally, don't force it.

### Check for context

At the start, quickly check what exists:
```bash
openspec list --json
```

This tells you:
- If there are active changes
- Their names, schemas, and status
- What the user might be working on

If the user mentioned a specific change name, read its artifacts for context.

### When no change exists

Think freely. When insights crystallize, you might offer:

- "This feels solid enough to start a change. Want me to create a proposal?"
- Or keep exploring - no pressure to formalize

### When a change exists

If the user mentions a change or you detect one is relevant:

1. **Resolve and read existing artifacts for context**
   - Run `openspec status --change "<name>" --json`.
   - Use `changeRoot`, `artifactPaths`, and `actionContext` from the status JSON.
   - Read existing files from `artifactPaths.<artifact>.existingOutputPaths`.

2. **Reference them naturally in conversation**
   - "Your design mentions using Redis, but we just realized SQLite fits better..."
   - "The proposal scopes this to premium users, but we're now thinking everyone..."

3. **Offer to capture when decisions are made**

    | Insight Type               | Where to Capture               |
    |----------------------------|--------------------------------|
    | New requirement discovered | `specs/<capability>/spec.md` |
    | Requirement changed        | `specs/<capability>/spec.md` |
    | Design decision made       | `design.md`                  |
    | Scope changed              | `proposal.md`                |
    | New work identified        | `tasks.md`                   |
    | Assumption invalidated     | Relevant artifact              |

   Example offers:
   - "That's a design decision. Capture it in design.md?"
   - "This is a new requirement. Add it to specs?"
   - "This changes scope. Update the proposal?"

4. **The user decides** - Offer and move on. Don't pressure. Don't auto-capture.

---

## What You Don't Have To Do

- Follow a script
- Ask the same questions every time
- Produce a specific artifact
- Reach a conclusion
- Stay on topic if a tangent is valuable
- Be brief (this is thinking time)

---

## Ending Discovery

There's no required ending. Discovery might:

- **Flow into a proposal**: "Ready to start? I can create a change proposal."
- **Result in artifact updates**: "Updated design.md with these decisions"
- **Just provide clarity**: User has what they need, moves on
- **Continue later**: "We can pick this up anytime"

When things crystallize, you might offer a summary - but it's optional. Sometimes the thinking IS the value.

---

## Guardrails

- **Don't implement** - Never write code or implement features. Creating OpenSpec artifacts is fine, writing application code is not.
- **Don't fake understanding** - If something is unclear, dig deeper
- **Don't rush** - Discovery is thinking time, not task time
- **Don't force structure** - Let patterns emerge naturally
- **Don't auto-capture** - Offer to save insights, don't just do it
- **Do visualize** - A good diagram is worth many paragraphs
- **Do explore the codebase** - Ground discussions in reality
- **Do question assumptions** - Including the user's and your own

We should extend the ai-providers to support providers that offer Qwen models (e.g. Qwen3.7 Max) and Minimax models (e.g. minimax-m3). This should be a small lift--we just need to make sure the TUI init supports these options. But, we also have the spec for "ai-providers" already created D:\repos\open-source\total-recall-04-opsx\openspec\specs\ai-provider\spec.md -- so we should review that for drift and then handle it here as well. I think we are calling this phase 4C but I wasn't able to find the source of truth for our current phase 4 status (what we have shipped, what is planned, etc.)

---

## Assistant (Build · Qwen3.7 Max · 8.2s)

_Thinking:_

The user wants to explore extending AI providers to support Qwen and Minimax models. They mention:
1. Reviewing the existing ai-providers spec for drift
2. Understanding the current phase 4 status
3. This is explore mode - thinking, not implementing

Let me start by investigating the codebase to understand the current state of things.

Let me dig into the current state of things — the spec, the existing provider code, and any phase tracking.



**Tool: skill**

---

## Assistant (Build · Qwen3.7 Max · 21.8s)

_Thinking:_

Good, I have the explore mode skill loaded. Now let me investigate the codebase to understand:
1. The current ai-providers spec
2. The existing provider implementation
3. Any phase tracking documentation
4. The TUI init code that handles provider selection

Let me start by reading the spec and checking for OpenSpec changes.


**Tool: bash**

**Tool: read**

**Tool: glob**

**Tool: glob**

---

## Assistant (Build · Qwen3.7 Max · 7.8s)

_Thinking:_

Good, I have the spec. Now let me look at:
1. The actual provider implementation to see what's currently supported
2. The TUI init code
3. Any documentation about phases

Let me search for phase information and look at the provider code.

**Tool: read**

**Tool: grep**

**Tool: glob**

**Tool: glob**

---

## Assistant (Build · Qwen3.7 Max · 6.9s)

_Thinking:_

Good, I found the ROADMAP.md which should have the phase information. Let me read that and also look at the provider implementation to understand what's currently supported.

**Tool: read**

**Tool: read**

**Tool: glob**

---

## Assistant (Build · Qwen3.7 Max · 6.8s)

_Thinking:_

Perfect! Now I have a good picture. Let me look at the TUI init code to see how providers are currently selected, and also check the wire.go factory to understand how providers are instantiated.

**Tool: glob**

**Tool: grep**

---

## Assistant (Build · Qwen3.7 Max · 6.6s)

_Thinking:_

Let me read the wire.go and main.go to understand how the TUI init works and how providers are selected.

**Tool: read**

**Tool: read**

---

## Assistant (Build · Qwen3.7 Max · 5.7s)

_Thinking:_

Let me also check the providerModelDefaults and providerAPIKeyPlaceholders maps to see what's currently configured.

**Tool: read**

---

## Assistant (Build · Qwen3.7 Max · 28.1s)

_Thinking:_

Perfect! Now I have a complete picture. Let me synthesize what I've learned:

**Current State:**
1. **Phase 4 Status:**
   - Phase 4A (MCP + Shell) is **shipped**
   - Phase 4B (VS Code Extension) is **next** (planned but not started)
   - Phase 4C (Answer Feedback) is **in progress** (there's an active change for it)

2. **AI Provider Spec:**
   - The spec at `openspec/specs/ai-provider/spec.md` lists these providers in the registry:
     - anthropic, openai, ollama, groq, lm-studio, custom
   - **No mention of Qwen or Minimax** in the spec

3. **Current Implementation:**
   - `internal/ai/provider.go` has the `ProviderRegistry` with: anthropic, openai, ollama, groq, lm-studio, custom
   - `cmd/total-recall/main.go` TUI shows these options in the select menu
   - `cmd/total-recall/wire.go` factory routes: anthropic → anthropic adapter, everything else → openai adapter (OpenAI-compatible)

4. **The Ask:**
   - Add support for Qwen models (e.g., Qwen3.7 Max)
   - Add support for Minimax models (e.g., minimax-m3)
   - These are likely OpenAI-compatible endpoints, so they'd use the openai adapter
   - Need to update the spec, registry, TUI options, and model defaults

**Key Questions to Explore:**
1. Are Qwen and Minimax OpenAI-compatible? (likely yes, they usually are)
2. What are their base URLs?
3. What are their model names?
4. Do they need API keys? (yes, cloud providers)
5. What environment variable names for API keys?


6. Should I update the spec to include these new providers?

Now I need to think through how to present these findings and what clarifying questions to ask the user about their implementation preferences.

Excellent. I've mapped the terrain. Let me show you what I found.

```
┌─────────────────────────────────────────────────────────────────┐
│                    PHASE 4 STATUS MAP                           │
└─────────────────────────────────────────────────────────────────┘

  Phase 4A ──────▶ Phase 4B ──────▶ Phase 4C
  MCP + Shell      VS Code Ext     Answer Feedback
    SHIPPED          PLANNED        IN PROGRESS
                                      │
                                      └─▶ Active change:
                                          "answer-feedback"
                                          (0/38 tasks done)
```

**The source of truth you're looking for:** `ROADMAP.md` lines 49-65

---

Now, about extending providers for Qwen and Minimax...

```
┌─────────────────────────────────────────────────────────────────┐
│              CURRENT PROVIDER ARCHITECTURE                      │
└─────────────────────────────────────────────────────────────────┘

  TUI Select Menu              ProviderRegistry           Adapter Route
  ───────────────              ────────────────           ─────────────
  Anthropic (Claude)    ───▶   "anthropic"         ───▶   anthropic.New()
  OpenAI (gpt-4o)       ───▶   "openai"            ───▶   openai.New()
  Groq                  ───▶   "groq"              ───▶   openai.New()
  Ollama (local)        ───▶   "ollama"            ───▶   openai.New()
  LM Studio (local)     ───▶   "lm-studio"         ───▶   openai.New()
  Custom (advanced)     ───▶   "custom"            ───▶   openai.New()
  
  Missing:
  ✗ Qwen (e.g., qwen3.7-max)
  ✗ Minimax (e.g., minimax-m3)
```

**Spec drift detected:** The spec at `openspec/specs/ai-provider/spec.md` line 29 lists:
> `anthropic`, `openai`, `ollama`, `groq`, `lm-studio`

But doesn't mention Qwen or Minimax.

---

**Questions to explore:**

1. **API compatibility:** Are Qwen and Minimax OpenAI-compatible? (I suspect yes — most cloud providers are these days)

2. **Base URLs:** What are their API endpoints?
   - Qwen: likely `https://dashscope.aliyuncs.com/compatible-mode/v1` or similar?
   - Minimax: likely `https://api.minimax.chat/v1` or similar?

3. **API keys:** Both need keys, yes? What env var names?
   - `DASHSCOPE_API_KEY` for Qwen?
   - `MINIMAX_API_KEY` for Minimax?

4. **Model names:** What are the canonical model identifiers?
   - Qwen: `qwen-max`, `qwen3.7-max`, `qwen-turbo`?
   - Minimax: `minimax-m3`, `abab6.5-chat`?

5. **Scope:** Is this just adding them to the registry + TUI, or do they need special handling (like Anthropic's JSON mode workaround)?

---

**The lift looks small:**
- Add 2 entries to `ProviderRegistry` in `internal/ai/provider.go`
- Add 2 options to the TUI select in `cmd/total-recall/main.go`
- Add 2 entries to `providerModelDefaults`
- Add 2 entries to `providerAPIKeyPlaceholders`
- Update the spec to reflect the new providers

All of these route through `openai.New()` since they're OpenAI-compatible.

Want me to verify the API details (base URLs, env vars, model names) before we proceed? Or do you have those handy?

---

## User

1. Yes, they are OpenAI-compatible, 2. Please verify their base URLs using the web or Context7, 3. Please use QWEN_API_KEY for Qwen models and MINIMAX_API_KEY for Minimax env var names, 4. Those canonical model identifiers look good except for abab6.5-chat I don't know what that is -- You should verify this with the web or Context7 as well, 5. Scope: the registry + TUI + docs and any initialization using openai.New() should be considered. Let's please call this phase 4D. So, yes please do the verification.

---

## Assistant (Build · Qwen3.7 Max · 14.7s)

_Thinking:_

Let me look up the API details for both Qwen and Minimax providers.

**Tool: context7_resolve-library-id**

**Tool: context7_resolve-library-id**

**Tool: webfetch**

**Tool: webfetch**

---

## Assistant (Build · Qwen3.7 Max · 11.6s)

_Thinking:_

Good, I confirmed Qwen's base URL. Now let me get Minimax's API details.

**Tool: webfetch**

**Tool: webfetch**

---

## Assistant (Build · Qwen3.7 Max · 8.3s)

_Thinking:_

The Minimax website didn't give me the specific API base URL. Let me try their developer platform docs directly.

**Tool: webfetch**

**Tool: webfetch**

---

## Assistant (Build · Qwen3.7 Max · 6.5s)

_Thinking:_

Interesting! Minimax has two API paths:
1. An Anthropic-compatible endpoint at `https://api.minimaxi.com/anthropic`
2. An OpenAI-compatible endpoint

Let me find the OpenAI-compatible base URL for Minimax.

**Tool: webfetch**

---

## Assistant (Build · Qwen3.7 Max · 6.5s)

_Thinking:_

Now let me get the specific OpenAI-compatible chat completions endpoint for Minimax.

**Tool: webfetch**

---

## Assistant (Build · Qwen3.7 Max · 19.6s)

_Thinking:_

I now have all the information I need. Let me summarize the findings.

**Qwen (Alibaba Cloud / DashScope):**
- Base URL: `https://dashscope.aliyuncs.com/compatible-mode/v1`
- OpenAI-compatible: Yes
- Env var: `DASHSCOPE_API_KEY` (user wants to use `QWEN_API_KEY`)
- Models: `qwen-plus`, `qwen-max`, `qwen-turbo`, etc.

**Minimax:**
- Base URL (OpenAI-compatible): `https://api.minimaxi.com/v1`


- OpenAI-compatible: Yes
- Env var: `MINIMAX_API_KEY`
- Models: `MiniMax-M3`, `MiniMax-M2.7`, `MiniMax-M2.5`, and others

Now I'm ready to present these findings.

Verification complete. Here's the full picture:

```
┌─────────────────────────────────────────────────────────────────────┐
│                    PHASE 4D — PROVIDER VERIFICATION                │
└─────────────────────────────────────────────────────────────────────┘

  Provider     Base URL (OpenAI-compatible)          Env Var           Default Model
  ─────────    ─────────────────────────────         ───────           ─────────────
  Qwen         https://dashscope.aliyuncs.com        QWEN_API_KEY      qwen-max
               /compatible-mode/v1

  Minimax      https://api.minimaxi.com/v1           MINIMAX_API_KEY   MiniMax-M3

  Both verified OpenAI-compatible ✓
  Both route through openai.New() ✓
```

**Qwen models available:** `qwen-max`, `qwen-plus`, `qwen-turbo`, `qwen3.7-max` (and others via DashScope)

**Minimax models available:** `MiniMax-M3`, `MiniMax-M2.7`, `MiniMax-M2.5`, `MiniMax-M2.1`, `MiniMax-M2` (plus `-highspeed` variants for M2.x)

**Note:** `abab6.5-chat` is a legacy model name — not present in current Minimax docs. `MiniMax-M3` is their flagship.

---

**Spec drift summary** (`openspec/specs/ai-provider/spec.md`):
- Registry list on line 29 is stale — missing `qwen` and `minimax`
- All other requirements (thin provider interface, JSON mode, nil-provider handling, no SDK) still hold — no drift there

**Phase 4 status** (from `ROADMAP.md`):

| Sub-phase | Name | Status |
|-----------|------|--------|
| 4A | MCP + Shell | Shipped |
| 4B | VS Code Extension | Planned (Next) |
| 4C | Answer Feedback | In Progress (0/38 tasks) |
| 4D | Qwen + Minimax Providers | **This change** |

---

**The lift is exactly what you expected:**

1. Add `"qwen"` and `"minimax"` to `ProviderRegistry` in `provider.go`
2. Add 2 TUI select options in `main.go`
3. Add entries to `providerModelDefaults` and `providerAPIKeyPlaceholders`
4. Update `wire.go` error message to include new providers
5. Update spec to reflect new registry entries
6. Update `ROADMAP.md` with Phase 4D entry

All route through `openai.New()` — no new adapter package needed.

Want me to create a change proposal for Phase 4D?

---

## User

Amazing job, before you create the change proposal for Phase 4D -- one more thing to consider: MiniMax looks like they provide their own model, which looks like you handle. Are we defaulting Qwen models to the dashscope provider? What about Alibaba Cloude Model Studio? That is the provider listed on the Opencode Go docs tables. Please tell me the implications of this. If dashscope and Alibaba Cloud Model Studio are distinct providers I would like to default to Alibaba I think... If it doesn't expand scope too much maybe we should support both but I actually don't know how to do that.

---

## Assistant (Build · Qwen3.7 Max · 16.6s)

_Thinking:_

The user is asking about the relationship between DashScope and Alibaba Cloud Model Studio. Let me think about what I know and what I found.

From the Aliyun documentation I fetched, I can see that:

1. **DashScope** is the API service name - it's the actual endpoint (`dashscope.aliyuncs.com`)
2. **Alibaba Cloud Model Studio** (阿里云百炼) is the product/platform name - it's the console/portal where you manage your API keys, models, billing, etc.

They are NOT distinct providers. They are the same service:
- Product name: 阿里云百炼 (Alibaba Cloud Model Studio / Bailian)


- API endpoint: `dashscope.aliyuncs.com/compatible-mode/v1`
- API Key: obtained from the Bailian console, commonly stored as `DASHSCOPE_API_KEY`

So "DashScope" is just the API layer of "Alibaba Cloud Model Studio". They're the same thing, just different names for different layers. The user mentioned "Alibaba Cloud Model Studio" appears in the OpenCode Go docs tables, so I should check what the opencode configuration looks like to see if there's any existing reference to these providers.

Good question. Let me check what OpenCode does with these, since you mentioned their docs.



**Tool: glob**

**Tool: glob**

**Tool: grep**

---

## Assistant (Build · Qwen3.7 Max · 5.9s)

_Thinking:_

No local config found. Let me check the OpenCode docs to see how they handle these providers.


**Tool: webfetch**

---

## Assistant (Build · Qwen3.7 Max · 8.1s)

_Thinking:_

Let me check the truncated output for the MiniMax and Qwen sections specifically.

**Tool: grep**

---

## Assistant (Build · Qwen3.7 Max · 8.6s)

_Thinking:_

I see that OpenCode lists MiniMax as a provider but I don't see a specific "Alibaba Cloud Model Studio" or "DashScope" provider listed. Let me search more specifically for any Alibaba-related providers.

**Tool: grep**

---

## Assistant (Build · Qwen3.7 Max · 8.0s)

_Thinking:_

So OpenCode's provider list doesn't have "Alibaba Cloud Model Studio" or "DashScope" or "Bailian" as a named provider. The user mentioned seeing it in "Opencode Go docs tables" - let me check the OpenCode Go section more carefully.

**Tool: read**

---

## Assistant (Build · Qwen3.7 Max · 7.1s)

_Thinking:_

Let me check the OpenCode Go section more carefully - the user mentioned seeing "Alibaba Cloud Model Studio" in the Go docs tables.


**Tool: webfetch**

---

## Assistant (Build · Qwen3.7 Max · 26.1s)

_Thinking:_

Now I have the full picture. Let me summarize what I found:

1. **DashScope** and **Alibaba Cloud Model Studio (Bailian)** are the SAME service:
   - "Alibaba Cloud Model Studio" (阿里云百炼 / Bailian) is the **product/platform name** - the console where you manage API keys, billing, etc.
   - "DashScope" is the **API endpoint name** - `dashscope.aliyuncs.com`
   - They are not distinct providers. DashScope is just the API layer of Alibaba Cloud Model Studio.
   
2. Looking at OpenCode Go's endpoints table, I see they're routing Qwen models through an Anthropic-compatible endpoint using the `@ai-sdk/anthropic` package, which is their own proxy layer. This is different from the native DashScope API, which uses OpenAI-compatible endpoints.

3. For Total Recall's purposes, I need to figure out which endpoint format makes sense given that we're using raw HTTP with OpenAI-compatible support.

The key insight is that DashScope and Alibaba Cloud Model Studio are the same service—DashScope is just the API endpoint name while Model Studio is the platform. Since we already have OpenAI-compatible infrastructure in place, DashScope's OpenAI-compatible endpoint is the natural choice. MiniMax offers both OpenAI and Anthropic compatibility, so either would work, but OpenAI-compatible keeps things consistent. This means there's no scope expansion needed—we're just using the same underlying service with the endpoint format that matches our existing setup.

Great question. Here's the answer:

```
┌─────────────────────────────────────────────────────────────────────┐
│         DashScope vs Alibaba Cloud Model Studio                     │
│                                                                     │
│         THEY ARE THE SAME SERVICE                                   │
└─────────────────────────────────────────────────────────────────────┘

  ┌──────────────────────────────────────────────────────────────┐
  │  Alibaba Cloud Model Studio (阿里云百炼 / Bailian)            │
  │                                                              │
  │  ┌─────────────────────────────────────────────────────┐     │
  │  │  Product / Platform Layer                           │     │
  │  │  • Console (bailian.console.aliyun.com)             │     │
  │  │  • API key management                               │     │
  │  │  • Billing, model marketplace                       │     │
  │  │  • This is what OpenCode Go docs call the provider  │     │
  │  └─────────────────────────────────────────────────────┘     │
  │                          │                                    │
  │                          ▼                                    │
  │  ┌─────────────────────────────────────────────────────┐     │
  │  │  API Layer                                          │     │
  │  │  • dashscope.aliyuncs.com/compatible-mode/v1        │     │
  │  │  • OpenAI-compatible Chat Completions               │     │
  │  │  • Auth: Bearer $DASHSCOPE_API_KEY                  │     │
  │  └─────────────────────────────────────────────────────┘     │
  └──────────────────────────────────────────────────────────────┘
```

**No scope expansion needed.** They're one service with two names:
- **Model Studio / Bailian** = the platform (where you sign up, get keys, manage billing)
- **DashScope** = the API endpoint (where HTTP requests go)

The OpenCode Go docs table lists "Alibaba Cloud Model Studio" as the provider name because that's the user-facing product. Under the hood, requests go to `dashscope.aliyuncs.com`.

**For Total Recall, the user experience is:**

```
TUI: "Qwen (Alibaba Cloud Model Studio)"
  → Provider key: "qwen"
  → Base URL: https://dashscope.aliyuncs.com/compatible-mode/v1
  → Env var: QWEN_API_KEY
  → Adapter: openai.New()
```

The TUI label can say "Alibaba Cloud Model Studio" for clarity (matching what users see in OpenCode docs), while the internal provider key stays `"qwen"` and the base URL is DashScope.

**One interesting thing I noticed in the OpenCode Go endpoints table:**

| Model | OpenCode Go Endpoint | SDK Package |
|-------|---------------------|-------------|
| Qwen3.7 Max | `opencode.ai/zen/go/v1/messages` | `@ai-sdk/anthropic` |
| MiniMax M3 | `opencode.ai/zen/go/v1/messages` | `@ai-sdk/anthropic` |

OpenCode Go routes both Qwen and MiniMax through an **Anthropic-compatible** endpoint (their own proxy). But that's OpenCode's internal routing choice — the native APIs for both are OpenAI-compatible, which is what we'd use directly.

**Bottom line:** One provider entry for Qwen, one for MiniMax, both OpenAI-compatible, no additional complexity. The TUI label for Qwen can reference "Alibaba Cloud" for discoverability.

Ready to create the Phase 4D proposal?

---

## User

Ah okay, perfect! This is a very good plan. Maybe we should extend for one more provider while we're at it. If it is easy to extend I would also like to support DeepSeek V4 Pro and DeepSeek V4 Flash from Deepseek provider. Please do the same research you did for the other options (Qwen and minimax). I know that Deepseek is OpenAI compatible but I don't know the base URL or model identifier. Env var should be called DEEPSEEK_API_KEY -- Everything else about your plan is perfect so don't change anything that isn't completely justified -- just add support for Deepseek as I have just described.

---

## Assistant (Build · Qwen3.7 Max · 12.1s)

_Thinking:_

The user wants me to research DeepSeek's API details - base URL, model identifiers, and confirm OpenAI compatibility. Let me fetch their docs.


**Tool: webfetch**

**Tool: webfetch**

---

## Assistant (Build · Qwen3.7 Max · 24.5s)

_Thinking:_

Perfect! I got the DeepSeek API details. Let me summarize what I found:

**DeepSeek API Details:**
- Base URL (OpenAI-compatible): `https://api.deepseek.com`
- Base URL (Anthropic-compatible): `https://api.deepseek.com/anthropic`
- Env var: `DEEPSEEK_API_KEY` (user specified this, matches their docs)
- Models available:
  - `deepseek-v4-flash` (new)
  - `deepseek-v4-pro` (new)
  - `deepseek-chat` (deprecated 2026/07/24)
  - `deepseek-reasoner` (deprecated 2026/07/24)

The user wants to support DeepSeek V4 Pro and DeepSeek V4 Flash specifically.

Key observations:
1. DeepSeek is OpenAI-compatible ✓
2. Base URL is `https://api.deepseek.com` (no `/v1` suffix - interesting!)
3. Model identifiers are `deepseek-v4-pro` and `deepseek-v4-flash`
4. They also support Anthropic API format at `/anthropic` endpoint
5. The old model names (`deepseek-chat`, `deepseek-reasoner`) are being deprecated on 2026/07/24

Now I need to verify this against what OpenCode Go shows. From the earlier fetch, I saw:
- OpenCode Go lists: "DeepSeek V4 Pro" and "DeepSeek V4 Flash"
- Their endpoint table shows these models routed through `opencode.ai/zen/go/v1/chat/completions` with `@ai-sdk/openai-compatible`

So DeepSeek is definitely OpenAI-compatible, which matches.

**Important note about the base URL:** DeepSeek's base URL is `https://api.deepseek.com` WITHOUT the `/v1` suffix. This is different from:
- OpenAI: `https://api.openai.com/v1`
- Qwen: `https://dashscope.aliyuncs.com/compatible-mode/v1`
- MiniMax: `https://api.minimaxi.com/v1`

This is a critical detail for the implementation. The OpenAI-compatible endpoint is at `https://api.deepseek.com/chat/completions`, not `https://api.deepseek.com/v1/chat/completions`.
 
Now I'm ready to document the Phase 4D proposal with all the verified provider details. I'll compile the complete specifications for Qwen, MiniMax, and DeepSeek, including their exact base URLs, environment variables, default models, and adapter implementations.

I need to update the spec at `openspec/specs/ai-provider/spec.md` to reflect these three new providers, then create the change proposal document that outlines the implementation plan and any necessary adjustments to the provider registry and configuration schema.


Perfect. DeepSeek details verified:

```
┌─────────────────────────────────────────────────────────────────────┐
│                    DEEPSEEK VERIFICATION                            │
└─────────────────────────────────────────────────────────────────────┘

  Provider     Base URL (OpenAI-compatible)          Env Var              Default Model
  ─────────    ─────────────────────────────         ───────              ─────────────
  DeepSeek     https://api.deepseek.com              DEEPSEEK_API_KEY     deepseek-v4-pro

  ⚠️ Note: No /v1 suffix on base URL (unlike most others)

  Models available:
  • deepseek-v4-pro (new, recommended)
  • deepseek-v4-flash (new, recommended)
  • deepseek-chat (deprecated 2026/07/24)
  • deepseek-reasoner (deprecated 2026/07/24)

  OpenAI-compatible ✓
  Also offers Anthropic-compatible endpoint at /anthropic
  Routes through openai.New() ✓
```

**Updated Phase 4D scope:**

| Provider | Base URL | Env Var | Default Model |
|----------|----------|---------|---------------|
| Qwen | `https://dashscope.aliyuncs.com/compatible-mode/v1` | `QWEN_API_KEY` | `qwen-max` |
| MiniMax | `https://api.minimaxi.com/v1` | `MINIMAX_API_KEY` | `MiniMax-M3` |
| DeepSeek | `https://api.deepseek.com` | `DEEPSEEK_API_KEY` | `deepseek-v4-pro` |

All three are OpenAI-compatible, all route through `openai.New()`, no new adapter packages needed.

Now creating the Phase 4D change proposal:



**Tool: skill**

---

## Assistant (Build · Qwen3.7 Max · 13.6s)

_Thinking:_

The user wants me to create a change proposal for Phase 4D - adding support for Qwen, MiniMax, and DeepSeek providers. I need to follow the openspec-propose skill workflow:

1. I have a clear description: "Add support for Qwen, MiniMax, and DeepSeek AI providers"
2. I should derive a kebab-case name like "add-qwen-minimax-deepseek-providers" or just "phase-4d-providers"
3. Create the change directory with `openspec new change`
4. Get status to see what artifacts are needed
5. Create artifacts in sequence

Let me start by creating the change. I'll use a descriptive name like "phase-4d-providers" or "add-qwen-minimax-deepseek".

Actually, looking at the existing changes, they use descriptive names like "answer-feedback", "mcp-delivery", etc. So I should use something like "qwen-minimax-deepseek-providers" or "phase-4d-ai-providers".

Let me go with "phase-4d-ai-providers" to be clear about the phase and what it's about.

**Tool: bash**

---

## Assistant (Build · Qwen3.7 Max · 7.0s)

_Thinking:_

Good, the change directory is created. Now I need to get the status to see what artifacts are required and in what order.

**Tool: bash**

---

## Assistant (Build · Qwen3.7 Max · 10.7s)

_Thinking:_

Good, I can see the artifact dependency order:
1. proposal (ready)
2. design (blocked by proposal)
3. specs (blocked by proposal)
4. tasks (blocked by design and specs)

I need to create them in this order: proposal → design + specs (can be parallel) → tasks

Let me get instructions for the proposal first.

**Tool: bash**

---

## Assistant (Build · Qwen3.7 Max · 19.7s)

_Thinking:_

Now I need to create the proposal. Let me think about the content:

**Why**: 
- Users want to use Qwen, MiniMax, and DeepSeek models for recall question generation
- These are popular OpenAI-compatible providers that should be first-class citizens in the registry
- Phase 4D extends provider support without adding complexity (all route through existing openai adapter)

**What Changes**:
- Add 3 new providers to ProviderRegistry: qwen, minimax, deepseek
- Add TUI select options for each in `tr init`
- Add model defaults and API key placeholders
- Update spec to reflect new registry entries
- Update ROADMAP.md with Phase 4D entry

**Capabilities**:
- New: None really - this is just extending existing provider support
- Modified: `ai-provider` - the registry list is changing (adding 3 new entries)

**Impact**:
- `internal/ai/provider.go` - ProviderRegistry map
- `cmd/total-recall/main.go` - TUI select options, providerModelDefaults, providerAPIKeyPlaceholders
- `cmd/total-recall/wire.go` - error message update
- `openspec/specs/ai-provider/spec.md` - registry list
- `ROADMAP.md` - Phase 4D entry
- No new dependencies, no breaking changes

Let me write this up.

**Tool: write**

---

## Assistant (Build · Qwen3.7 Max · 9.8s)

_Thinking:_

Good, proposal is created. Now I need to get the status again to see what's next, then create design and specs (which can be done in parallel since they both depend only on proposal).


**Tool: bash**

---

## Assistant (Build · Qwen3.7 Max · 8.8s)

_Thinking:_

Good, proposal is done. Now design and specs are both ready (unblocked). Let me get instructions for both and create them. I'll do design first, then specs.


**Tool: bash**

**Tool: bash**

---

## Assistant (Build · Qwen3.7 Max · 8.2s)

_Thinking:_

Good, I have instructions for both design and specs. Let me create them now.

For design, I need to explain:
- Context: current provider system, why these 3 providers
- Goals/Non-Goals: what we're doing vs not doing
- Decisions: why these specific base URLs, model defaults, env var names, etc.
- Risks/Trade-offs: any concerns

For specs, I need to create a delta spec for the modified `ai-provider` capability. The proposal says we're modifying the ai-provider spec to add 3 new registry entries. I need to look at the existing spec and create a MODIFIED requirement for the registry.

Let me read the existing ai-provider spec first to understand what needs to be modified.

**Tool: read**

---

## Assistant (Build · Qwen3.7 Max · 30.0s)

_Thinking:_

Now I can see the existing spec. The requirement I need to modify is "Named provider registry resolves base URLs" on line 29. It currently lists: `anthropic`, `openai`, `ollama`, `groq`, `lm-studio`. I need to add `qwen`, `minimax`, and `deepseek` to that list.

Let me create both the design and specs artifacts now.

For design, I'll focus on:
- Context: current state of provider system
- Goals: add 3 new providers with minimal complexity
- Non-Goals: not adding new adapters, not supporting provider-specific features
- Decisions: base URLs, model defaults, env var names, TUI labels
- Risks: base URL changes, model deprecation

For specs, I'll create a MODIFIED requirement for the registry, adding the 3 new providers.

**Tool: write**

---

## Assistant (Build · Qwen3.7 Max · 8.3s)

_Thinking:_

Good, design is created. Now I need to create the specs artifact. The proposal says we're modifying the `ai-provider` capability, so I need to create a delta spec at `specs/ai-provider/spec.md` with a MODIFIED requirement for the registry.

Let me create the specs directory and file.

**Tool: bash**

---

## Assistant (Build · Qwen3.7 Max · 15.1s)

_Thinking:_

Good, directory created. Now I need to write the delta spec. I'm modifying the "Named provider registry resolves base URLs" requirement to add the 3 new providers. I need to copy the entire requirement block from the existing spec and modify it.


**Tool: write**

---

## Assistant (Build · Qwen3.7 Max · 9.2s)

_Thinking:_

Good, specs are created. Now I need to check status again and then create the tasks artifact, which is the final one needed for implementation.

**Tool: bash**

---

## Assistant (Build · Qwen3.7 Max · 8.0s)

_Thinking:_

Perfect! All three artifacts (proposal, design, specs) are done. Now I need to create the tasks artifact, which is the final one required for implementation. Let me get the instructions for tasks.


**Tool: bash**

---

## Assistant (Build · Qwen3.7 Max · 17.6s)

_Thinking:_

Now I need to create the tasks list. Let me think about what needs to be done:

1. Update ProviderRegistry in internal/ai/provider.go - add 3 new entries
2. Update providerModelDefaults in cmd/total-recall/main.go - add 3 new entries
3. Update providerAPIKeyPlaceholders in cmd/total-recall/main.go - add 3 new entries
4. Update TUI select options in cmd/total-recall/main.go - add 3 new options
5. Update TUI flow case in cmd/total-recall/main.go - add qwen, minimax, deepseek to the cloud provider case
6. Update wire.go error message to include new providers
7. Update openspec/specs/ai-provider/spec.md - update the registry requirement
8. Update ROADMAP.md - add Phase 4D section
9. Build and test

Let me organize these into logical groups.


**Tool: write**

---

## Assistant (Build · Qwen3.7 Max · 8.1s)

_Thinking:_

Good, tasks are created. Now let me check the final status to confirm everything is ready for implementation.

**Tool: bash**

---

## Assistant (Build · Qwen3.7 Max · 11.2s)

_Thinking:_

Perfect! All artifacts are complete. Now I should summarize what was created and let the user know they can proceed to implementation.

**Phase 4D change proposal created!** ✓

**Change:** `phase-4d-ai-providers`  
**Location:** `openspec/changes/phase-4d-ai-providers/`

**Artifacts created:**
- **proposal.md** — Why we're adding Qwen, MiniMax, and DeepSeek providers
- **design.md** — Technical decisions (base URLs, model defaults, env vars, TUI labels)
- **specs/ai-provider/spec.md** — Delta spec adding 3 new registry entries with scenarios
- **tasks.md** — 11 implementation tasks across 7 groups

**Summary:**
```
Provider   Base URL                                          Default Model      Env Var
────────   ────────                                          ─────────────      ───────
Qwen       https://dashscope.aliyuncs.com/compatible-mode/v1 qwen-max           QWEN_API_KEY
MiniMax    https://api.minimaxi.com/v1                       MiniMax-M3         MINIMAX_API_KEY
DeepSeek   https://api.deepseek.com                          deepseek-v4-pro    DEEPSEEK_API_KEY
```

All three route through `openai.New()` — no new adapter packages needed.

**Ready for implementation!** Run `/opsx-apply` or ask me to implement to start working on the tasks.

---

