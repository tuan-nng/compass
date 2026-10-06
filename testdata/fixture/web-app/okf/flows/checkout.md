---
type: Flow
title: Checkout flow
description: Cart to paid order in three steps.
tags: [web, checkout]
status: stable
generated: { by: claude-code/opus-5, at: 2026-06-20T11:00:00Z }
---
# Checkout flow

1. The cart page totals the order.
2. The pay step creates an invoice through the billing API.
3. The receipt page polls the invoice until it is paid.
