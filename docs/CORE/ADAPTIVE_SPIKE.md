## Adaptive Difficulty Spike

running script: ./scripts/e2e/adaptive-difficulty-ab.sh
output dir: /tmp/tmp.xnjJlmaBPk/.tr

```
scratch dir: /tmp/tmp.xnjJlmaBPk
data dir:    /tmp/tmp.xnjJlmaBPk/.tr
copied /home/colin/.tr/config.yaml into the scratch data dir
scratch daemon running (pid 235596)
=== arm: ai-delegation ===
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
captured 5/10 questions -> /tmp/tmp.xnjJlmaBPk/.tr/arms/ai-delegation.txt
=== arm: high-cluster ===
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
captured 7/10 questions -> /tmp/tmp.xnjJlmaBPk/.tr/arms/high-cluster.txt
=== arm: dispersed ===
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
captured 1/10 questions -> /tmp/tmp.xnjJlmaBPk/.tr/arms/dispersed.txt
=== arm: all-code ===
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
captured 0/10 questions -> /tmp/tmp.xnjJlmaBPk/.tr/arms/all-code.txt
=== arm: fallback ===
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
  (no question within 30s)
captured 0/10 questions -> /tmp/tmp.xnjJlmaBPk/.tr/arms/fallback.txt

Resolver selections (daemon log):
2026/09/24 15:55:01 [recall] adaptive resolver selected "hard" (signals: ai-delegation)
2026/09/24 15:55:24 [recall] adaptive resolver selected "hard" (signals: ai-delegation)
2026/09/24 15:56:22 [recall] adaptive resolver selected "hard" (signals: ai-delegation)
2026/09/24 15:56:53 [recall] adaptive resolver selected "hard" (signals: ai-delegation)
2026/09/24 15:57:18 [recall] adaptive resolver selected "hard" (signals: ai-delegation)
2026/09/24 15:57:50 [recall] adaptive resolver selected "hard" (signals: ai-delegation)
2026/09/24 15:58:11 [recall] adaptive resolver selected "hard" (signals: ai-delegation)
2026/09/24 15:58:42 [recall] adaptive resolver selected "hard" (signals: ai-delegation)
2026/09/24 15:59:07 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 15:59:33 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 15:59:52 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:00:18 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:00:37 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:01:07 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:01:12 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:02:35 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:02:49 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:03:26 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:04:02 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:05:03 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:05:30 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:06:00 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:06:32 [recall] adaptive resolver selected "hard" (signals: high-cluster)
2026/09/24 16:07:13 [recall] adaptive resolver selected "hard" (signals: high-cluster)

Per-arm question captures: /tmp/tmp.xnjJlmaBPk/.tr/arms/<arm>.txt
Compare question quality side-by-side, then record observations in
docs/CORE/ADAPTIVE_SPIKE.md (see openspec change adaptive-difficulty, task 5.2).
Scratch dir left for inspection: /tmp/tmp.xnjJlmaBPk
```

Note: All adaptive resolver selections are "hard" -- this is undesirable
Note: 2 groups captured 0 questions, resulting in 3/5 arms *.txt file outputs -- undesirable
Note: ~1/3rd output is written in Chinese (probably due to small free LLM) -- directly vioaltes "same language" rule (this rule exists in a policy somewhere...)
Note: many of the questions directly involve "jitter", "exponential backoff", "retries", etc. which are explicitly referred to as NOT to be used repeatedly -- there is an example in the policy doc and the small LLM's use it incorectly, assuming this is the domain knowledge that should be quizzed (ALL of the high-cluster.txt questions/choices are on this topic...)

This in general is a FAIL. The script is working but the LLM is bad. Needs a better LLM before true evaluation.
