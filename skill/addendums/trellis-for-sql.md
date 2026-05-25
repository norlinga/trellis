# Trellis For SQL

This addendum is advisory.
The canonical Trellis skill owns handle semantics.

## Handle Shape

SQL handles should name durable data contracts: tables, views, materialized views, migrations, stored procedures, functions, jobs, or report outputs.

Useful patterns:

```trellis
Provides:
  - Table:billing.invoices
  - View:analytics.monthly_revenue
  - Procedure:billing.apply_payment
  - Migration:create_invoices_table
  - Report:finance.monthly_revenue
```

Typed prefixes are often clearer than plain dotted paths because SQL objects can share names across object kinds.

## Source Anchors

Good anchors:

```trellis
Provides:
  - Table:billing.invoices @source("label:create_invoices")
  - View:analytics.monthly_revenue @source("symbol:monthly_revenue")
  - Procedure:billing.apply_payment @source("symbol:apply_payment")
```

Use `line:<start>-<end>` when the SQL file has no stable labels or symbols.

## Consumes

List data contracts whose schema or semantics affect this SQL unit:

```trellis
Consumes:
  - Table:billing.customers
  - Table:billing.payments
  - View:analytics.active_accounts
```

Usually omit:

- every column reference
- built-in SQL functions
- temporary CTE names internal to the query
- indexes unless the contract is performance-sensitive and index availability is intentionally part of the unit's contract

## Common Mistakes

Avoid:

```trellis
Provides:
  - SELECT
  - invoices
  - id
```

Prefer:

```trellis
Provides:
  - Table:billing.invoices
  - View:analytics.invoice_summary
```

