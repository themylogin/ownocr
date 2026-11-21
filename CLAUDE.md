## Writing rules (ASD-STE100 based)

All prose (docs, specs, proposals, commit messages, chat) follows
Simplified Technical English principles:

- Max 20 words per sentence for instructions, 25 for descriptions.
- Active voice, present tense. "The parser rejects X", not
  "X will be rejected by the parser".
- One instruction per sentence. One topic per paragraph.
- Use simple, common words. Prefer "use" over "utilize", "start"
  over "initiate", "help" over "facilitate".
- No noun clusters longer than 3 nouns.
- State facts directly. No hedging ("might potentially", "it could
  be argued").

## Comment rules

- Default: zero comments. Code must explain itself through names.
- Comments allowed ONLY for: non-obvious invariants, workarounds
  with a link/reason, "why" that the code cannot express, warnings
  about surprising behavior.
- Never: comments restating the code ("// increment counter"),
  section banners, docstrings on trivial/private functions,
  TODO without an owner, commented-out code.
- If a comment explains WHAT the code does, delete the comment.
