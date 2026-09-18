---
description: Review the current changes with the code-reviewer subagent.
agent: build
---

Use the `code-reviewer` subagent to review the current uncommitted changes
(`git diff`, plus untracked files). Pass it the full diff as context. Scope
the review to `$ARGUMENTS` if specific files/paths are given. Report the
findings and the recommended fixes back to the user; do not apply fixes unless
asked.