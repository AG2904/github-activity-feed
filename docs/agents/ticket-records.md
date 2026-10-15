# Ticket records: Local Markdown

When a ticket is complete, its work is recorded as one document at
`docs/tickets/<feature-slug>/<NN>-<slug>.md` (same number and slug as the
ticket file under `.scratch/<feature-slug>/issues/`).

The ticket under `.scratch/` is the **plan and acceptance criteria**; the record
under `docs/tickets/` is the **evidence of what was built** — what changed, how
it behaves now, and how it was verified. Same shape as the records kept in the
Programa-Tuberias repo (Spanish, professional register).

## Required shape

```markdown
# Ticket <NN> - <Título>

## Resumen
<what the ticket is and, if the work happened in stages, which PRs/comments>

## Cambios Realizados
### <archivo o área>
<what changed in it, and why when non-obvious>

## Comportamiento resultante
<observable behavior after the change>

## Verificación
<execution evidence: suite counts, typecheck, lint; the exact commands>

## Acceptance Criteria
- ✅ AC1: <criterion, verbatim or paraphrased from the ticket>
```

## Rules

- Write it when the ticket's acceptance criteria are all met and the work is
  merged (or ready to merge) — not before.
- Quote real outputs, never invented ones; if something was not verified, say so
  explicitly (as the Programa-Tuberias record does for docs-only tickets).
- The Acceptance Criteria list mirrors the ticket checklist; every criterion is
  ✅ or an explicit gap.
- Accepted deviations, judgement calls, and follow-ups belong in the relevant
  section, not silently omitted.
