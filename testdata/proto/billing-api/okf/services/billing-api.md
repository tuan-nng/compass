---
type: Service
title: Billing API
description: Go service that creates invoices and charges customers.
tags: [billing]
status: stable
generated: { by: claude-code/opus-5, at: 2026-06-10T09:00:00Z }
verified: { by: human:alice, at: 2026-06-12T16:00:00Z }
sources:
  - id: main-go
    resource: https://github.com/acme/billing-api/blob/main/cmd/billing-api/main.go
---
# Billing API

Serves the [invoice API](../contracts/invoice-api.md). The idempotency
middleware runs before the payment call, so a repeated key never reaches the
payment provider.[^main-go]

[^main-go]: Middleware order is set in `cmd/billing-api/main.go`.
