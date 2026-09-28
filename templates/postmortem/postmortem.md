# Postmortem: <short title>

- **Date:** <YYYY-MM-DD>
- **Author:** <name>
- **Status:** draft | reviewed
- **Severity:** <low | medium | high>
- **Duration:** <detection to full recovery>

## Summary

Two or three sentences: what broke, who or what was affected, and how it ended.

## Impact

What users or downstream systems experienced. Include numbers from the benchmark report or dashboards (error rate, latency percentiles, requests dropped) and say whether they are physical measurements or virtual (simulated) values.

## Timeline

All times in one time zone. One line per event; include how each was detected.

| Time | Event |
|---|---|
| hh:mm | First symptom (metric, log, or alert) |
| hh:mm | Detection |
| hh:mm | Mitigation started |
| hh:mm | Recovery confirmed |

## Root cause

What actually caused the failure, traced to a mechanism (not a person). Distinguish the trigger from the underlying weakness.

## Detection

Which signals fired, which did not, and how long the gap between failure and detection was.

## Response

What was tried, what worked, and what wasted time.

## What went well

-

## What went poorly

-

## Where we got lucky

-

## Action items

Each item needs an owner and is either preventive, detective, or mitigating.

| Action | Type | Owner | Tracking |
|---|---|---|---|
| | | | |

## Lessons

What this incident teaches about the system, and which exercise or scenario in `docs/scenarios.md` would rehearse it.
