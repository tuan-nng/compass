---
type: Gotcha
title: Invoice creation needs an idempotency key
description: POST /v2/invoices double-charges on retry without Idempotency-Key.
tags: [billing, retries]
status: draft
generated: { by: cursor/gpt-5.6, at: 2026-06-01T12:00:00Z }
stale_after: 2026-09-01T00:00:00Z
---
# Invoice creation needs an idempotency key

Retrying `POST /v2/invoices` after a timeout creates a second invoice and
charges the customer twice, unless the request carries an `Idempotency-Key`
header. Generate the key once per logical invoice and reuse it on every retry.

See the [invoice API contract](/contracts/invoice-api.md).
