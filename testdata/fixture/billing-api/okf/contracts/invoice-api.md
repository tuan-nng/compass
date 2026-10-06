---
type: API Contract
title: Invoice API v2
description: REST contract for creating and reading invoices.
tags: [billing, api]
resource: https://github.com/acme/billing-api/blob/main/api/openapi.yaml
generated: { by: claude-code/opus-5, at: 2026-06-15T10:00:00Z }
verified: { by: process:contract-test, at: 2026-09-20T08:00:00Z }
stale_after: 2026-12-31T00:00:00Z
---
# Invoice API v2

`POST /v2/invoices` creates an invoice and charges the customer's default
payment method. `GET /v2/invoices/{id}` reads one back.

Clients must send an `Idempotency-Key` header on every create. The server keeps
keys for 24 hours and returns the first response for a repeated key.
