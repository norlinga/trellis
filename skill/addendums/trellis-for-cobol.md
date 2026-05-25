# Trellis For COBOL

This addendum is advisory.
The canonical Trellis skill owns handle semantics.

## Handle Shape

COBOL sidecars often need to describe stable business contracts rather than only raw paragraph names.
Prefer handles for decisions, records, copybooks, program entry points, file layouts, batch steps, and externally meaningful workflow behavior.

Useful patterns:

```trellis
Provides:
  - Program:BillingWorkflow.main
  - Decision:BillingWorkflow.discountEligibility
  - Record:CustomerAccount
  - Copybook:CustomerAccountLayout
  - Batch:NightlyBilling.applyPayments
```

Use typed prefixes when the kind matters:

- `Program:`
- `Decision:`
- `Record:`
- `Copybook:`
- `Batch:`
- `File:`
- `Event:`

Do not turn every paragraph into a handle. Paragraphs and sections are often best represented as source anchors.

## Source Anchors

Good anchors:

```trellis
Provides:
  - Decision:BillingWorkflow.discountEligibility @source("label:DISCOUNT-ELIGIBILITY")
  - Program:BillingWorkflow.main @source("label:PROCEDURE-DIVISION")
  - Record:CustomerAccount @source("label:CUSTOMER-ACCOUNT-RECORD")
```

Use `label:<name>` for paragraphs, sections, copybook labels, and record declarations.
Use line anchors only when labels are absent or unreliable.

## Consumes

List:

- copybooks whose layout changes would force this program to change
- called programs or transaction boundaries
- files, records, or reports whose format is part of the contract
- decisions from other programs that this unit relies on

Usually omit:

- every paragraph performed internally
- temporary working-storage fields
- incidental MOVE/COMPUTE details
- raw JCL plumbing unless it is the contract being modeled

## Common Mistakes

Avoid:

```trellis
Provides:
  - CALC-RATE
  - line.248
  - BillingWorkflow.CALC-RATE
```

Prefer:

```trellis
Provides:
  - Decision:BillingWorkflow.rateSelection @source("label:CALC-RATE")
```

The handle names the decision.
The anchor names where the current code implements it.

