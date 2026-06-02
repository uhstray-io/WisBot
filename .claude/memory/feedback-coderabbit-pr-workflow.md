---
name: feedback-coderabbit-pr-workflow
description: All code merges go through a PR validated by CodeRabbit before merge
metadata:
  node_type: memory
  type: feedback
---

Never merge code straight to `main`. Every change ships via a **pull request** so **CodeRabbit** can review it. **Resolve every finding before merging — but you don't have to wait for CodeRabbit's formal re-approval.**

- **Ordinary findings** (not major/catastrophic — no large rewrite): push the fix + reply, ensure **CI is green**, then **merge after the fact**. Do NOT wait for CodeRabbit to re-review/flip its flag (its re-review often lags behind a transient fair-usage **rate limit**, and burning ~10 min re-triggering just to clear a stale `CHANGES_REQUESTED` on already-resolved minor findings is wasted time).
- **Major / catastrophic findings** (big rewrites, security-critical): wait for CodeRabbit to re-review and confirm before merging.
- A **reasoned push-back** on an incorrect finding counts as resolving it. Never merge over a genuinely *unresolved* finding.

**Why:** (updated 2026-06-02 per user) CodeRabbit on PRs is the validation gate, but the formal-approval flag is often blocked by rate limits even when findings are fixed — so for minor issues, fixing + green CI is sufficient to merge.

**How to apply:** branch from `main` → implement → push → open PR → let CodeRabbit + CI run → fix findings (push + reply) → merge once the fix is in and CI is green (squash + delete branch). Only block the merge on a CodeRabbit re-review for major findings. Phased work = one branch/PR per phase. Related: [[feedback-no-coauthors]], [[project-agent-cloud-deployment]].
