# Trellis For .NET

This addendum is advisory.
The canonical Trellis skill owns handle semantics.

## Handle Shape

.NET handles should usually follow namespace/type/member boundaries, while avoiding unnecessarily long fully qualified names when the workspace has a clearer convention.

Useful patterns:

```trellis
Provides:
  - Billing.InvoiceService.CreateInvoice
  - Billing.InvoiceRepository.FindOpenInvoices
  - Api.InvoicesController.Create
  - Domain.InvoiceCreated
  - Job:Billing.ReconcileAccounts
```

For interfaces, name the stable contract when the interface is the meaningful boundary:

```trellis
Provides:
  - PaymentGateway.Charge
```

If multiple implementations exist and each matters separately, include adapter or implementation ownership:

```trellis
Provides:
  - StripePaymentGateway.Charge
  - OfflinePaymentGateway.Charge
```

## Source Anchors

Good anchors:

```trellis
Provides:
  - Billing.InvoiceService.CreateInvoice @source("symbol:CreateInvoice")
  - Api.InvoicesController.Create @source("symbol:Create")
  - Domain.InvoiceCreated @source("symbol:InvoiceCreated")
```

Use `symbol:<name>` for classes, records, methods, handlers, controllers, minimal API handlers, commands, queries, and events.

## Consumes

List:

- application services, repositories, handlers, commands, queries, events, and domain types whose contracts matter
- DTOs, records, options, and configuration contracts when they shape this unit's interface
- framework APIs only when their behavior or signature is load-bearing
- external packages when changes would force corresponding edits here

Usually omit:

- BCL incidentals such as `string`, `DateTime`, `IEnumerable`, `Task`, or LINQ methods
- private methods in the same class
- every method on a dependency already represented by a service/interface handle
- dependency-injection wiring unless the registration is the contract being modeled

## Framework Notes

For ASP.NET controllers:

```trellis
Provides:
  - Api.InvoicesController.Create
  - Api.InvoicesController.Get
```

For minimal APIs:

```trellis
Provides:
  - Api.BillingInvoices.Create
```

For MediatR-style handlers:

```trellis
Provides:
  - Command:CreateInvoice
  - Query:GetOpenInvoices
  - Handler:CreateInvoice
```

For background services or jobs:

```trellis
Provides:
  - Job:Billing.ReconcileAccounts
```

## Common Mistakes

Avoid:

```trellis
Provides:
  - Create
  - Handle
  - Controllers.InvoicesController.cs.line42
  - POST /api/invoices
```

Prefer:

```trellis
Provides:
  - Api.InvoicesController.Create @source("symbol:Create")
  - Handler:CreateInvoice @source("symbol:Handle")
```

