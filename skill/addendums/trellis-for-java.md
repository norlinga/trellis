# Trellis For Java

This addendum is advisory.
The canonical Trellis skill owns handle semantics.

## Handle Shape

Java handles should usually follow package/class/member boundaries, but avoid overly long full package names when the workspace has a clearer convention.

Useful patterns:

```trellis
Provides:
  - billing.InvoiceService.createInvoice
  - billing.InvoiceRepository.findOpenInvoices
  - api.InvoiceController.create
  - domain.InvoiceCreated
  - config.BillingJobConfig
```

For interfaces, name the contract rather than one implementation:

```trellis
Provides:
  - PaymentGateway.charge
```

An implementation can provide the same contract only if it is the single owner in the workspace.
Otherwise use handles that distinguish adapter ownership:

```trellis
Provides:
  - StripePaymentGateway.charge
```

## Source Anchors

Good anchors:

```trellis
Provides:
  - billing.InvoiceService.createInvoice @source("symbol:createInvoice")
  - api.InvoiceController.create @source("symbol:create")
  - domain.InvoiceCreated @source("symbol:InvoiceCreated")
```

Use `symbol:<methodOrClass>` for classes, methods, records, and enums.

## Consumes

List:

- application services, repositories, interfaces, event types, DTOs, and domain types whose contracts matter
- framework annotations or base types only when they shape the unit's public contract

Usually omit:

- JDK incidentals such as `List`, `Optional`, `String`, `Collectors`
- private methods
- common framework plumbing unless modeled elsewhere

## Common Mistakes

Avoid:

```trellis
Provides:
  - create
  - com.company.project.billing.internal.InvoiceService.createInvoice
```

Prefer:

```trellis
Provides:
  - billing.InvoiceService.createInvoice @source("symbol:createInvoice")
```

