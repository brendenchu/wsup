---
name: diagnose-issue
description: Investigate a reported defect or performance regression using a reproducible signal. Use when behaviour fails or differs from expectations.
---

# Diagnose an issue

Identify the owner and read its AGENTS.md. Separate expected behaviour, observed
behaviour and environment. Preserve current changes before experimenting.

Establish a repeatable signal using an existing test, a minimal input, a captured
request with private data removed, or a targeted command. Keep raw captures and
disposable output in the owner's ignored `workbench/.local/`. Record findings
with provenance (source revision, input identity, reproduction command and
relevant environment), and preserve historical capture paths and hashes by
recording relocations rather than rewriting them. Observe the actual
failure before treating a hypothesis as its cause. For intermittent failures,
record frequency and conditions rather than claiming deterministic reproduction.

Compare plausible causes against the signal, changing one factor at a time. Inspect
callers, contracts and installed versions where relevant. Keep probes scoped;
shared services and destructive experiments need explicit authorisation.

When a cause is supported, make the smallest coherent repair within the requested
scope. Add regression coverage at a seam exercising the real defect when feasible.
Re-run the original signal and relevant adjacent checks. Remove temporary probes.

Report the supported cause, repair, before/after evidence and remaining limits.
If reproduction is blocked, report the precise missing input or access without
presenting an untested hypothesis as a confirmed diagnosis.
