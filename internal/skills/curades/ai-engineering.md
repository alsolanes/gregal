---
name: ai-engineering
title: AI engineering
description: Use to design, implement, evaluate, or operate LLM and machine-learning features, agents, retrieval, tool use, model routing, and AI application infrastructure.
---
# AI engineering

Treat model behavior as a probabilistic system inside a product, not as a prompt-only problem. Start from the user outcome, failure cost, data boundary, and latency/cost budget.

1. Use deterministic code for deterministic work. Add a model only where flexible language, perception, or judgment provides measurable value.
2. Begin with the simplest architecture that can meet the requirement. Add retrieval, tools, agents, memory, or model routing only for a demonstrated need; define limits and handoff conditions.
3. Build a small representative evaluation set from real success and failure cases before tuning prompts or switching models. Measure task success, failure modes, latency, and cost; rerun it after changes.
4. Inspect traces across model calls, tool calls, retries, and handoffs. Make tool schemas narrow, validate outputs, bound retries and context, and preserve cancellation and error states.
5. Treat retrieved text, files, tool output, and user content as untrusted input. Keep authorization in code, separate instructions from data, minimize secrets and permissions, and require review for consequential actions.
6. Plan for provider timeouts, malformed output, rate limits, empty results, and model changes. Use typed validation, explicit fallbacks, observability, and graceful degradation.
7. Measure quality against the actual product objective. Avoid claiming that a prompt change, benchmark, or model label guarantees safety or correctness.
