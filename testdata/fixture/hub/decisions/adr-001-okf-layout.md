---
type: Decision
title: One OKF bundle per repo plus a hub
description: Each repo owns its bundle; cross-repo edges live only in the hub.
tags: [okf, layout]
status: stable
generated: { by: claude-code/opus-5, at: 2026-09-20T09:00:00Z }
---
# One OKF bundle per repo plus a hub

Each repo keeps its knowledge next to its code. Edges between repos, such as
the [invoice dependency](../cross-repo/invoice-dependency.md), live in the hub,
the only bundle that sees every repo.
