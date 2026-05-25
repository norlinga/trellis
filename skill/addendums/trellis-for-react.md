# Trellis For React

This addendum is advisory.
The canonical Trellis skill owns handle semantics.

## Handle Shape

React handles should describe component contracts, hooks, user-visible actions, state transitions, and data-loading boundaries.

Useful patterns:

```trellis
Provides:
  - Component:Billing.InvoiceTable
  - Component:Billing.InvoiceTable.render
  - Hook:useInvoiceFilters
  - Action:InvoiceTable.selectInvoice
  - Route:BillingInvoicesPage
```

Use typed prefixes when they clarify what kind of contract is being named.

For component files, prefer a handle for the component's user-facing contract, not every helper function in the file.

## Source Anchors

Good anchors:

```trellis
Provides:
  - Component:Billing.InvoiceTable @source("symbol:InvoiceTable")
  - Hook:useInvoiceFilters @source("symbol:useInvoiceFilters")
  - Action:InvoiceTable.selectInvoice @source("symbol:handleSelectInvoice")
```

## Consumes

List:

- app hooks and components whose contract matters
- API/data contracts consumed by the component
- state machines, context providers, or route contracts

Usually omit:

- every imported UI primitive
- CSS modules/classes
- incidental icons
- React built-ins such as `useState` or `useEffect`
- test IDs and DOM details unless they are the declared contract

## Common Mistakes

Avoid:

```trellis
Provides:
  - Rendered
  - div.table
  - onClick
```

Prefer:

```trellis
Provides:
  - Component:Billing.InvoiceTable @source("symbol:InvoiceTable")
  - Action:InvoiceTable.selectInvoice @source("symbol:handleSelectInvoice")
```

