# Memory Index

- [No co-authors in commits](feedback-no-coauthors.md) — never add Co-Authored-By trailers to commit messages
- [agent-cloud deployment](project-agent-cloud-deployment.md) — WisBot deploys via agent-cloud as a pulled image; secrets in OpenBao, site values in site-config
- [CodeRabbit PR workflow](feedback-coderabbit-pr-workflow.md) — all merges go through a PR; merge only when CodeRabbit fully passes; resolve findings + re-review first
- [Deployment phase progress](project-deployment-phase-progress.md) — Milestone A (phases 1–4) merged + image published; phases 5–7 (agent-cloud + site-config) remain
- [2026-06-05 review fixes](project-2026-06-05-review-fixes.md) — 23 findings from the multi-agent review fixed in PRs #23–#25; UTC "O"-string + idempotent-Start invariants; CodeRabbit rate-limit handling
- [2026-06-07 security audit](project-2026-06-07-security-audit.md) — 2 high + 7 medium fixed (#27–#30); recording authz/consent, AllowedMentions, upload quota/rate-limit invariants; 22 low deferred
- [Go prototype retired](project-go-prototype-retired.md) — Go reimplementation is dead; C# main is the direction forward; dead branches + Dependabot go PRs cleaned up (2026-06-22)
