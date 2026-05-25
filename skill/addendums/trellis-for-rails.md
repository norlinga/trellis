# Trellis For Rails And Ruby

This addendum is advisory.
The canonical Trellis skill owns handle semantics.

## Handle Shape

Use Trellis dotted names, not Ruby syntax. Convert `::` and `#` into dots, then include enough namespace to be globally unique.

| Ruby/Rails unit | Preferred handle |
|---|---|
| Controller action | `Api.V1.FilesController.create` |
| Service object method | `BillingCheckoutService.create_session` |
| Model contract | `Team.current_bandwidth_usage` |
| Concern method | `Api.ErrorHandling.render_not_found_error` |
| ViewComponent class method | `Ui.CardComponent.card_classes` |
| ERB/ViewComponent template | `Ui.CardComponentView.render` |
| Helper method | `FilesHelper.file_size_label` |
| Job | `FormSubmissionDigestJob.perform` |
| Stimulus controller action | `CollectionMembershipController.add` |

## Source Anchors

Good anchors:

```trellis
Provides:
  - Api.V1.FilesController.create @source("symbol:create")
  - BillingCheckoutService.create_session @source("symbol:create_session")
  - Ui.CardComponentView.render @source("template:render")
```

For ERB templates, anchor to the template or meaningful DOM section only if the codebase has a stable convention for doing so.

## Consumes

List app contracts and framework APIs only when their contract is load-bearing.

Good:

```trellis
Consumes:
  - BillingCheckoutService.create_session
  - Api.ErrorHandling.render_not_found_error
```

Usually omit:

- route helpers
- instance variables and assigns
- partial names
- `ApplicationController`
- Active Record query helpers
- framework exception classes

Put incidental framework assumptions in `Invariants:` or scenario steps.

## Common Mistakes

Avoid:

```trellis
Provides:
  - GET /api/v1/files
  - Api::V1::FilesController#create
  - Rendered
```

Prefer:

```trellis
Provides:
  - Api.V1.FilesController.create @source("symbol:create")
  - Api.V1.FilesIndexView.render @source("template:render")
```

