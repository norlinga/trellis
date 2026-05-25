# Trellis For TypeScript And JavaScript

This addendum is advisory.
The canonical Trellis skill owns handle semantics.

## Handle Shape

Use handles that match exported modules, functions, classes, types, events, and runtime contracts.

Useful patterns:

```trellis
Provides:
  - billing.calculateProration
  - billing.InvoiceService.createInvoice
  - Type:Billing.InvoiceDTO
  - Event:invoice.created
  - ApiClient:Billing.fetchInvoices
```

For frontend code, pair this with the React addendum when React conventions are more specific.

## Source Anchors

Good anchors:

```trellis
Provides:
  - billing.calculateProration @source("symbol:calculateProration")
  - billing.InvoiceService.createInvoice @source("symbol:createInvoice")
  - Type:Billing.InvoiceDTO @source("symbol:InvoiceDTO")
```

Use `symbol:<name>` for exported functions, classes, interfaces, types, and constants.

## Consumes

List:

- exported app functions, classes, hooks, and types whose contract matters
- API client methods and generated data contracts
- event names or message contracts that other units depend on
- external libraries only when their contract is load-bearing

Usually omit:

- language built-ins and DOM APIs
- incidental utility imports
- local helpers private to the same module
- every property access on a type already listed

## Common Mistakes

Avoid:

```trellis
Provides:
  - index
  - default
  - handleClick
```

Prefer:

```trellis
Provides:
  - billing.calculateProration @source("symbol:calculateProration")
  - Action:InvoiceEditor.saveDraft @source("symbol:handleSaveDraft")
```

