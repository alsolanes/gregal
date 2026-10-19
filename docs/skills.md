# Specialist skills

Gregal ships five optional specialists: `anti-ai-slop`, `data-science`,
`statistics`, `ai-engineering`, and `product-design`. They start enabled for a
new installation and can be switched independently in **Settings → Agent →
Specialist skills**. The selection is stored in `skills.enabled` in the Gregal
configuration. Existing user and project skills keep working as before.

Only the short name and routing description of enabled skills are added to the
prompt. The agent opens a full skill with the `skill` tool when its task matches
the description. Disabled specialists are omitted from discovery and the tool
will not return their content. This keeps long reference text out of unrelated
turns; the small routing descriptions are the only always-present overhead.

After a successful, non-autonomous task, Gregal may suggest a new skill if the
work surfaced a concrete project-specific or recurring workflow that is not
already captured elsewhere. It shows a brief draft and asks before saving; it
does not turn one-off requests, generic advice, or sensitive information into
skills. Agreeing to a suggestion lets the agent save it as a project skill or
personal skill in a follow-up turn.

## Why these skills

The skills are original, compact workflows adapted from authoritative practice
and public references, not copied skill bundles. They aim to make the agent
check evidence, assumptions, and rendered results rather than apply a generic
template.

- **Anti-slop review:** judge visible output quality and specificity, never
  authorship. The third-party [anti-ai-slop skill](https://github.com/ch040602/anti-ai-slop)
  is a conceptual reference for evidence-led critique and actionable fixes.
- **Data science:** use train/test boundaries and pipelines to prevent data
  leakage, following the [scikit-learn common pitfalls guide](https://scikit-learn.org/stable/common_pitfalls).
- **Statistics:** report effect size and uncertainty, and interpret p-values
  within the model and assumptions, guided by the [American Statistical
  Association statement](https://www.amstat.org/asa/files/pdfs/p-valuestatement.pdf).
- **AI engineering:** start with simple composable systems and evaluate actual
  traces and representative datasets, drawing on [Anthropic's agent engineering
  guidance](https://www.anthropic.com/engineering/building-effective-agents) and
  [OpenAI's agent evaluation guide](https://developers.openai.com/api/docs/guides/agent-evals).
- **Product design:** preserve existing design systems, design for real user
  tasks, and verify accessibility and interaction states with [W3C WCAG 2.2](https://www.w3.org/TR/WCAG22/)
  as a reference.
