---
type: Cross-Repo Dependency
title: web-app depends on the billing invoice API
description: web-app checkout calls POST /v2/invoices in billing-api.
tags: [billing, web]
status: stable
generated: { by: claude-code/opus-5, at: 2026-09-20T09:00:00Z }
verified: { by: human:alice, at: 2026-09-21T09:00:00Z }
---
# web-app depends on the billing invoice API

The [web app](/repos/web-app/services/web-app.md) calls the
[invoice API](/repos/billing-api/contracts/invoice-api.md) during checkout.

Breaking changes to the contract need a web-app release first.
