---
type: Service
title: Web app
description: Customer-facing web app; checkout creates invoices and signs users in.
tags: [web, billing]
status: stable
generated: { by: claude-code/opus-5, at: 2026-06-20T11:00:00Z }
---
# Web app

The [checkout flow](../flows/checkout.md) calls the billing API's
[invoice contract](../../billing-api/contracts/invoice-api.md) and
authenticates the user with a
[session token](../../shared-auth/concepts/session-token.md).

Billing source: [billing-api on GitHub](https://github.com/acme/billing-api).
