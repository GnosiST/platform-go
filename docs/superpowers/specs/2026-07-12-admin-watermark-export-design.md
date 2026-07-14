# Admin Watermark And Export Design

## Goal

Replace the single fixed screen watermark with one backward-compatible control model that supports screen and export scopes plus exact visual watermark counts.

The watermark is a presentation and provenance aid, not a security boundary. It cannot prevent screenshots, DOM modification or direct API use. A deployment that requires mandatory watermarks needs a separate server-enforced policy; this phase preserves user-selectable behavior.

## Configuration Contract

```ts
type WatermarkCount = 1 | 4 | 9 | 16;
type WatermarkScope = "screen" | "export";

type AdminUIConfig = {
  watermark: boolean;
  watermarkCount: WatermarkCount;
  watermarkScopes: WatermarkScope[];
};
```

Defaults are `watermark=false`, `watermarkCount=1` and `watermarkScopes=["screen"]`. Persisted or imported legacy objects containing only `watermark` normalize to these defaults. Invalid counts or scopes are discarded without breaking the remaining UI settings.

## Settings Experience

Keep one watermark section in `SystemSettingsDrawer`:

- one master switch;
- when enabled, a checkbox group for screen and export application scopes;
- when screen scope is active, a labeled segmented control for `1`, `4`, `9` and `16`;
- persistent helper text explaining that export behavior depends on the export format;
- synchronized Chinese and English copy.

Controls must meet 44px touch targets at mobile widths, expose visible labels, remain keyboard operable and preserve focus when progressive options appear.

## Screen Rendering

Render one dedicated `aria-hidden="true"` watermark layer directly under `.platform-shell`. It is fixed to the viewport with `inset:0`, sits above the shell and Ant Design overlays, and keeps `pointer-events:none`, so the marks cover the topbar, sidebar, data lists, dashboard, drawers, dropdowns and modals without intercepting interaction.

Generate exactly the configured number of spans. Desktop uses `1x1`, `2x2`, `3x3` or `4x4` CSS grids and places first/last rows and columns against viewport edges so chrome is visibly marked. At `<=768px`, the sixteen-mark mode reflows to `2x8` with a fixed smaller text size and tighter gaps; attribution text must remain readable without horizontal overflow. Each item rotates independently.

## Export Semantics

### Policy Review JSON

The first supported export scope is policy-review evidence JSON. The request explicitly states whether export watermark metadata is requested. The result remains valid JSON and adds structured metadata containing `applied`, product identifier, exported actor and export time. Audit data records only whether watermark metadata was applied, not a duplicated free-form watermark string.

### Canonical OpenAPI

`GET /api/openapi.json` and the canonical downloaded OpenAPI document are never watermarked or replaced by a marked derivative. The generated contract must still evolve when the API adds the policy-review watermark query and response metadata; this schema update is not watermarking the OpenAPI artifact.

### Original Files

Original file downloads remain byte-identical. PDF or image watermarking requires an explicit derived-copy endpoint and is outside this phase. CSV and XLSX require format-specific metadata rules before support.

Deferred formats are explicit: this phase does not claim generic CSV, XLSX, PDF, image or arbitrary-file watermark support. Each future format requires a separate contract for provenance, byte preservation, authorization, storage and retention.

## Acceptance

- Legacy localStorage and imported JSON settings normalize correctly.
- All scope combinations persist and restore.
- The DOM contains exactly `1`, `4`, `9` or `16` hidden watermark items.
- Screen marks cover navigation, data surfaces and body-portaled overlays while all controls remain operable.
- Sixteen marks reflow to two columns on narrow viewports without truncated attribution text.
- Mobile and desktop layouts have no horizontal overflow in light and dark themes.
- Policy-review JSON stays valid and carries accurate structured metadata.
- Watermark settings never create or replace a marked OpenAPI derivative, and original downloaded file bytes do not change.
- The enabled policy-review Admin OpenAPI accurately describes the watermark query and response metadata.
- UI contract drift tests fail when counts, compatibility normalization, accessibility hiding or grid rules are removed.
