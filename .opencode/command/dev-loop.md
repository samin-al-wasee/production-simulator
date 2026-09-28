---
description: Run one pass of the development loop (plan, implement, review, test, document) for a requirement.
agent: build
---

Run one pass of the ForgeLab development loop for this requirement:

$ARGUMENTS

Read `docs/development-loop.md` and `AGENTS.md`, then follow the loop steps in
order: requirement, plan, implement, review, test, document. Present the plan
and wait for approval before implementing anything non-trivial. Use the
`code-reviewer` subagent for the review step and `make check` for the test
step. End with the exit-criteria checklist and stop at the checkpoint; do not
commit, push, or start another pass unless asked.
