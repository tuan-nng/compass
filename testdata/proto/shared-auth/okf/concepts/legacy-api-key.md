---
type: Concept
title: Legacy API key
description: Static per-client API key auth, replaced by the session token.
tags: [auth, deprecated]
status: deprecated
generated: { by: claude-code/opus-5, at: 2026-05-02T08:00:00Z }
---
# Legacy API key

Static keys sent in `X-Api-Key`. New clients use the
[session token](session-token.md) instead; remaining keys are revoked at the
end of the migration.
