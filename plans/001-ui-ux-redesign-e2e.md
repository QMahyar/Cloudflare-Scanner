# UI/UX redesign — E2E implementation plan

Status: complete

## Goals

- Make the app feel like a focused desktop network utility rather than a long web form.
- Preserve every existing workflow, API contract, scan state, RTL language, and theme.
- Improve hierarchy, information density, scan setup flow, help discoverability, and responsive behavior.
- Keep Svelte 5 as the UI implementation layer; avoid new runtime dependencies.

## Direction

A compact operations console with:

- a persistent desktop sidebar containing product identity, navigation, live local-server state, and utilities;
- a calm, wide content canvas with a page context header;
- task-first cards with clearer setup groups and sticky primary actions;
- contextual help that remains useful without competing with the task;
- a compact bottom dock and efficient single-column flow on mobile;
- stronger surface separation and typography while retaining Cloudflare orange as the signal color.

## Work

1. Audit current screenshots, Svelte structure, tokens, interaction state, RTL, and breakpoints.
2. Redesign the app shell and navigation in `App.svelte`.
3. Rework global tokens and responsive layout in `tokens.css` / `app.css`.
4. Refine scanner and replacer workbench hierarchy without changing behavior.
5. Add only the i18n strings required by the shell/page context, in English and Persian.
6. Validate keyboard navigation, focus treatment, mobile overflow, light/dark theme, and RTL.
7. Run frontend tests/build, Go vet/tests/build, then launch the real binary and run browser-driven E2E smoke checks against it at desktop and mobile viewports.
8. Record final changed files, test results, screenshots, and any limitations.

## Acceptance criteria

- All four tabs remain mounted and preserve state.
- Desktop UI has no nested page scrollbar caused by the help pane.
- Primary scan controls and status are visually obvious.
- 320, 375, 414, 768, 1280, and 1920 px widths have no unintended horizontal page overflow.
- English/Persian and light/dark modes render correctly.
- Tab navigation works by click and arrow/Home/End keys.
- Existing frontend and Go test suites pass; production UI and Go binary build successfully.
- Browser E2E can visit every tab and exercise safe form/navigation/theme/language interactions without console errors.

## Completion record

- Frontend Vitest: 11/11 passed.
- Production Vite build: passed.
- `go vet ./...`: passed.
- `go test ./...`: passed.
- Stripped Windows binary build: passed.
- Real compiled binary launched with bundled xray sidecar.
- Browser E2E: all tabs, keyboard tab navigation, dark/light, English/Persian RTL, and overflow checks passed at 1920, 1280, 768, 414, 375, and 320 px.
- Workflow E2E: parsed a VLESS config, generated two replaced configs through the Go API, completed a one-address clean-IP scan through SSE, completed a one-endpoint native scanner run, and loaded version data.
- Browser console/page errors: none.
- Review screenshots: `e2e-artifacts/`.
