---
type: Service
title: Web app
description: Customer-facing web app; checkout creates invoices and signs users in.
tags: [web, billing]
status: stable
generated: { by: claude-code/opus-5, at: 2026-06-20T11:00:00Z }
---
# Web app

The [checkout flow](../flows/checkout.md) calls the billing API's invoice
contract and authenticates the user with a shared-auth session token.

Other repos, for human readers:
[billing-api](https://github.com/acme/billing-api),
[shared-auth](https://github.com/acme/shared-auth). The hub records how
web-app depends on them.
