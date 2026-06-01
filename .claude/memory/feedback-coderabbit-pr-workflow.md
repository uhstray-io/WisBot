---
name: feedback-coderabbit-pr-workflow
description: All code merges go through a PR validated by CodeRabbit before merge
metadata:
  node_type: memory
  type: feedback
---

Never merge code straight to `main`. Every change ships via a **pull request** so **CodeRabbit** can review it. Merge **only when validation fully passes**. If CodeRabbit raises an issue, **resolve it and let CodeRabbit re-review** (push the fix, wait for the new review) **before merging** — don't merge over unresolved findings.

**Why:** CodeRabbit review on PRs is the team's validation gate; merging without it (or over open findings) skips the safety check the user relies on.

**How to apply:** branch from `main` → implement → push → open PR (`gh pr create`) → wait for **all** checks (CodeRabbit + CI) → if findings, fix + push + wait for re-review → only merge once approved/green. Squash-merge + delete branch is the user's chosen merge style. Phased work = one branch/PR per phase. Related: [[feedback-no-coauthors]], [[project-agent-cloud-deployment]].
