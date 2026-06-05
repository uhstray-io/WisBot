---
name: project-2026-06-05-review-fixes
description: Outcome of the 2026-06-05 comprehensive multi-agent review — all 23 findings fixed in PRs #23–#25
metadata:
  node_type: memory
  type: project
---

A 37-agent adversarially-verified review (7 dimensions: core runtime, voice, web/upload
security, data services, deployment, live local-dev, docs) confirmed 23 findings (0
critical, 1 high, 7 medium, 15 low; 1 refuted). All fixed and merged 2026-06-05 in three
severity-ordered PRs: **#23** (high: stale /wisllm docs), **#24** (medium: graceful
shutdown via PosixSignalRegistration + Bot.StopBot, /voicestats lastActive bug, recording
drain-window truncation, download Content-Type sanitization + nosniff, reminder requeue,
deploy-o11y marked non-functional, wisbot.db* gitignored), **#25** (low: DB-init ordering,
UTC "O"-string invariant doc, mkdir DbPath, voice save isolation/sanitized filenames,
slim /health, pre-stream StatObject probe, stale-'uploading' reset, Ollama 404 vs
unreachable, compose.yaml CI validation + image boot smoke test before publish, docs).

**Notable invariants now relied on** (don't break):
- DateTime columns store `DateTime.UtcNow.ToString("O")` ONLY — due-time queries compare
  lexicographically as TEXT (documented in Database.cs header).
- `StartRetention()` / service Start methods must stay idempotent — OnReady re-fires on
  every Discord reconnect.
- compose.yaml's `.env` is `required: false` so `docker compose config` validates in CI.
- build-and-publish boots the image (dummy token → /health must answer 503/200) before
  pushing any tag.

**Process learnings:** CodeRabbit rate-limits PR reviews per hour (per [[feedback-coderabbit-pr-workflow]]) —
a silent stall on PR creation is usually the limit; check issue comments for the
rate-limit notice and re-trigger with `@coderabbitai review` after the window. Gate
commits on `dotnet build`'s own exit code (piping to `tail` masks failures).
