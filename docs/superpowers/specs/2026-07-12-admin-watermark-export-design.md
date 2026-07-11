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

Render a dedicated `aria-hidden="true"` watermark layer. Generate exactly the configured number of spans and arrange them as `1x1`, `2x2`, `3x3` or `4x4` CSS grids. Each item rotates independently. The layer keeps `pointer-events:none`, stable containment and a lower stacking level than interactive content.

Responsive rules reduce watermark text size and gaps for dense mobile grids without viewport-based font scaling. Long product or user names wrap or clip inside their cell and must not cause horizontal page overflow.

## Export Semantics

### Policy Review JSON

The first supported export scope is policy-review evidence JSON. The request explicitly states whether export watermark metadata is requested. The result remains valid JSON and adds structured metadata containing `applied`, product identifier, exported actor and export time. Audit data records only whether watermark metadata was applied, not a duplicated free-form watermark string.

### Canonical OpenAPI

`GET /api/openapi.json` and the canonical downloaded OpenAPI document remain unchanged. A future marked derivative may use a standards-compliant extension, but it must not replace the canonical artifact.

### Original Files

Original file downloads remain byte-identical. PDF or image watermarking requires an explicit derived-copy endpoint and is outside this phase. CSV and XLSX require format-specific metadata rules before support.

Deferred formats are explicit: this phase does not claim generic CSV, XLSX, PDF, image or arbitrary-file watermark support. Each future format requires a separate contract for provenance, byte preservation, authorization, storage and retention.

## Acceptance

- Legacy localStorage and imported JSON settings normalize correctly.
- All scope combinations persist and restore.
- The DOM contains exactly `1`, `4`, `9` or `16` hidden watermark items.
- Mobile and desktop layouts have no horizontal overflow in light and dark themes.
- Policy-review JSON stays valid and carries accurate structured metadata.
- Canonical OpenAPI bytes and original downloaded file bytes do not change.
- UI contract drift tests fail when counts, compatibility normalization, accessibility hiding or grid rules are removed.
