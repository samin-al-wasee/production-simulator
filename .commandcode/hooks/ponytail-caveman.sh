#!/usr/bin/env bash
# Command Code hook: inject the always-on ponytail and caveman modes.
#
# Reads the installed skill bodies (so it stays in sync with the skills):
#   - SessionStart: inject both full rulesets into the first turn.
#   - PreToolUse (write|edit): reinforce the ponytail ladder before code changes.
set -euo pipefail

event="${COMMANDCODE_HOOK_EVENT:-}"
project="${COMMANDCODE_PROJECT_DIR:-$PWD}"

# Print a SKILL.md body with its leading YAML frontmatter removed.
skill_body() {
  local file="$project/.commandcode/skills/$1/SKILL.md"
  [ -f "$file" ] || return 0
  awk 'BEGIN { f = 0 }
       f >= 2 { print; next }
       /^---[[:space:]]*$/ { f++; next }
       f == 0 { print }' "$file"
}

case "$event" in
  SessionStart)
    ctx=$(printf 'Two modes are always on for this session. Apply both.

=== PONYTAIL (always on) ===
%s

=== CAVEMAN (always on) ===
%s' "$(skill_body ponytail)" "$(skill_body caveman)")
    jq -n --arg ctx "$ctx" '{
      hookSpecificOutput: {
        hookEventName: "SessionStart",
        additionalContext: $ctx
      }
    }'
    ;;
  PreToolUse)
    ctx=$(printf 'Ponytail is always on: before writing or editing code, stop at the first rung that holds (YAGNI, reuse, stdlib, native, dependency, one line, minimum). Never cut validation, error handling, security, or accessibility.

%s' "$(skill_body ponytail)")
    jq -n --arg ctx "$ctx" '{
      hookSpecificOutput: {
        hookEventName: "PreToolUse",
        permissionDecision: "allow",
        additionalContext: $ctx
      }
    }'
    ;;
  *)
    exit 0
    ;;
esac
