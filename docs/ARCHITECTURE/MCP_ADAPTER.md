## Exposes Tools

Something like:

```

generate_recall_question()       ← pulls from cache, no diff needed

get_active_concepts()            ← what's live right now?

get_knowledge_state(concept)     ← how well does the user know X?

get_active_concepts()            ← what's in the cache right now?

get_knowledge_state(concept)     ← how well do they know X?

get_concept_weaknesses()         ← where are the gaps?

generate_socratic_prompt(topic)  ← let the agent ask the question (not sure about this one)

evaluate_recall_answer(answer)   ← how well did they do?

record_interaction_event(...)    ← track agent-mediated learning

record_commit_event()            ← track commit context for later recall opportunities

```

That’s mostly orchestration plumbing.