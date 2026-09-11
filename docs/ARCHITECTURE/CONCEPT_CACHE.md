## Cache Aggressively

Do NOT hit the model API constantly.

Especially because:

- developers save files frequently
- diffs mutate rapidly
- staging changes incrementally

Use:

- diff hashing
- concept fingerprints
- incremental invalidation
- incremental concept tracking
- diff fingerprinting
- concept reuse
- cache invalidation
- staged cognition reuse

Example:

```
Same retry logic concept already analyzed
	->
reuse concept cache
```

This will massively reduce:

- latency
- token cost
- API spam

## Hint

- chunk intelligently
- analyze incrementally
- avoid whole-repo prompts