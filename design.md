# Design — Cloudflare Scanner

A locked design system for this app. Every page redesign reads this file before emitting code. Extend this system instead of inventing a per-page theme.

## Genre

Modern-minimal, technical operations tooling.

## Audience And Job

- Audience: technical users familiar with WARP, proxy configurations, endpoints, and scan settings.
- Primary job: configure and start a scan quickly, then review and export results.
- Tone: precise, calm, technical, and dense only where the data requires it.

## Macrostructure Family

- App pages: Workbench. Configuration and primary action lead; progress and results follow in the same persistent workspace.
- Content pages: compact Long Document treatment inside the same shell.
- Navigation: N3 side rail on desktop; labelled bottom dock on small screens.
- Footer: none inside the operational workspace.

## Theme

- `--color-paper`: `oklch(13% 0.012 55)`
- `--color-paper-2`: `oklch(16% 0.014 55)`
- `--color-paper-3`: `oklch(20% 0.014 55)`
- `--color-ink`: `oklch(94% 0.008 75)`
- `--color-ink-2`: `oklch(77% 0.012 70)`
- `--color-rule`: `oklch(29% 0.014 55)`
- `--color-accent`: `oklch(70% 0.19 50)`
- `--color-focus`: `oklch(80% 0.16 60)`

Cloudflare orange is a signal, not a field color. It marks active navigation, focus, progress, and the primary action edge. Surfaces are opaque warm charcoal with precise rules. Glass, colored blooms, decorative gradients, and glow-heavy shadows are not part of the system.

## Typography

- Display: Bahnschrift or the platform's variable display sans, weight 700, normal style.
- Body: Aptos / Segoe UI Variable Text / Noto Sans, weight 400.
- Mono: Cascadia Mono / JetBrains Mono / SFMono, weight 500.
- Persian: Tahoma / Noto Sans Arabic.
- Data uses tabular numerals.
- Headings are roman. Italic display type is not used.

The app intentionally uses local font stacks so the embedded, local-only interface remains dependable without an external font request.

## Spacing

Use the 4-point named scale in `tokens.css`. Controls use named tokens; raw spacing values are reserved for one-pixel optical corrections only.

## Motion

- Motion communicates state only: button press, active navigation, progress, modal, and toast.
- No page or card reveal animations.
- Use `--ease-out`, `--ease-in`, and `--ease-in-out`.
- Reduced-motion removes spatial movement and keeps short opacity changes.

## Microinteractions Stance

- Focus rings appear immediately.
- Primary buttons lift by at most one pixel on fine pointers and return on press.
- Cards do not lift or glow.
- Scan activity may pulse; decorative infinite motion is prohibited.
- Success feedback is quiet; existing toasts remain for async confirmations and failures.

## CTA Voice

- Primary: compact solid ink button with an orange signal edge; direct verb label.
- Secondary: graphite surface with a clear rule.
- Destructive: reserved red surface, never competing with the primary action.

## Responsive Behavior

- Desktop at 60rem: fixed side rail and a broad workbench canvas.
- Tablet and phone: top utility header plus a persistent labelled bottom navigation dock.
- Verify at 320, 375, 414, and 768 CSS pixels.
- Controls remain at least 44px tall on coarse pointers.
- Result tables scroll horizontally with readable type; they do not compress into illegible columns.
- Use logical properties for RTL behavior.

## Per-page Allowances

- Scanner pages prioritize configuration, the start action, progress, then results.
- Replacer may use denser multi-step groups because its two workflows are explicit.
- About is typography-led and uses no enrichment.
- App pages use no decorative hero imagery.

## What Pages Must Share

- Wordmark, orange signal color, typography, control geometry, focus treatment, navigation, surface hierarchy, and spacing scale.
- All four pages remain mounted so active scans and local component state survive navigation.

## Exports

The canonical CSS export is `tokens.css` at the project root.
