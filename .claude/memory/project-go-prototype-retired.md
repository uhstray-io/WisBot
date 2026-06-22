---
name: project-go-prototype-retired
description: The Go reimplementation of WisBot is dead; the C# .NET app on main is the de facto direction
metadata:
  node_type: memory
  type: project
---

There was an experimental **Go reimplementation of WisBot** (`module wisbot`, Go 1.25.6:
`bwmarrin/discordgo` + `gofiber/fiber`/`danielgtaylor/huma` + `jackc/pgx`/`pgvector-go` +
`tmc/langchaingo` + OpenTelemetry — i.e. the vector-DB / LLM / observability track). It lived
only on stale side branches and never had a `go.mod` on `main`.

**Decision (2026-06-22): retired.** The C# .NET app on `main` is the de facto direction forward.
Dead branches `go`, `vectordb`, `observability-test`, `aarens` were deleted from origin (tip SHAs
recorded in this session if recovery is ever needed). Three zombie Dependabot go-module PRs
(#4 x/net, #6 x/crypto, #7 fiber) were closed — they targeted `main` (no `go.mod`) so could never
merge. The current `.github/dependabot.yml` on main has **no gomod ecosystem**, so no new Go PRs
will spawn.

**Why:** Go track abandoned; maintaining two implementations + dead-end Dependabot noise wasn't worth it.

**How to apply:** Don't resurrect the Go prototype or treat those Dependabot PRs/branches as live
work. If vector-DB/LLM/o11y features are wanted, build them into the C# app (see Phase 8 file-relay
precedent in [[project-deployment-phase-progress]]).
