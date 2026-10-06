---
type: Concept
title: Session token
description: Signed session token that shared-auth issues to signed-in users.
tags: [auth, session]
status: stable
generated: { by: claude-code/opus-5, at: 2026-05-02T08:00:00Z }
verified: { by: human:bob, at: 2026-05-04T10:00:00Z }
---
# Session token

shared-auth signs a session token at sign-in. Services check its signature
locally with the published key set and never call shared-auth per request.
