# Project Rules

## Prompt Logging

Every time you receive a new instruction or prompt, append a new entry to `prompts.txt` in the project root. Create the file if it doesn't exist, and keep this log updated throughout all sessions.

Each entry MUST contain all three of the following:

1. **Timestamp** — in ISO 8601 format (e.g. `2026-06-12T14:30:00Z`).
2. **PROMPT** — the user's instruction/prompt, recorded **verbatim**. This is required, not optional. Do not replace it with a paraphrase or summary. If the prompt is very long, you may truncate the middle, but keep the opening and closing text intact and mark the cut with `[...]`.
3. **SUMMARY** — a brief description of what you did in response.

Use this exact format for every entry, separating entries with a line of `=` characters:

```
========================================================================
<ISO 8601 timestamp>
------------------------------------------------------------------------
PROMPT:
<the user's prompt, verbatim>

SUMMARY:
<brief summary of what you did>
========================================================================
```

An entry that omits the verbatim PROMPT is incorrect and must be fixed.

<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan at
`specs/001-feature-voting-system/plan.md` (Option A — Synchronous Atomic
Increments). Companion artifacts: `research.md`, `data-model.md`,
`contracts/openapi.yaml`, and `quickstart.md` in the same directory.
<!-- SPECKIT END -->
