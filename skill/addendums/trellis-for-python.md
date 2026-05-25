# Trellis For Python

This addendum is advisory.
The canonical Trellis skill owns handle semantics.

## Handle Shape

Python handles should usually follow module/class/function boundaries, while staying focused on stable contract names rather than every helper function.

Useful patterns:

```trellis
Provides:
  - billing.proration.calculate
  - billing.invoice_service.create_invoice
  - billing.InvoiceRepository.find_open_invoices
  - Task:billing.reconcile_accounts
  - Event:invoice.created
```

Use module-qualified function names for module functions.
Use `Module.Class.method` for class methods when the class is part of the contract.
Use typed prefixes when the contract is an event, task, command, or data shape rather than a direct Python symbol.

## Source Anchors

Good anchors:

```trellis
Provides:
  - billing.proration.calculate @source("symbol:calculate")
  - billing.InvoiceService.create_invoice @source("symbol:create_invoice")
  - Task:billing.reconcile_accounts @source("symbol:reconcile_accounts")
```

Use `symbol:<name>` for functions, classes, methods, dataclasses, Pydantic models, Celery tasks, FastAPI route handlers, and Django views.
Use line anchors only when the source has no stable symbol.

## Consumes

List:

- app services, repositories, tasks, commands, views, and domain functions whose contracts matter
- data models that shape this unit's interface, such as dataclasses, Pydantic models, serializers, or DTO-like objects
- framework APIs only when their contract is load-bearing for this unit
- external library APIs when version or semantic changes would force edits here

Usually omit:

- stdlib incidentals such as `pathlib.Path`, `datetime`, `json.loads`, or `logging`
- private helpers in the same module
- every method on a dependency already represented by a type or service handle
- framework plumbing such as decorators unless the decorator semantics are the contract being modeled

## Framework Notes

For Django:

```trellis
Provides:
  - BillingInvoiceView.get
  - BillingInvoiceSerializer.validate
  - BillingInvoiceQuerySet.open
```

For FastAPI:

```trellis
Provides:
  - Api.BillingInvoices.create
  - Api.BillingInvoices.list
```

For Celery or background jobs:

```trellis
Provides:
  - Task:billing.reconcile_accounts
```

## Common Mistakes

Avoid:

```trellis
Provides:
  - process
  - __call__
  - views.py.line42
  - POST /billing/invoices
```

Prefer:

```trellis
Provides:
  - Api.BillingInvoices.create @source("symbol:create_invoice")
  - billing.InvoiceProcessor.process @source("symbol:process")
```

