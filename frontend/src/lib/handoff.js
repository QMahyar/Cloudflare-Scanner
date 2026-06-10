import { writable } from 'svelte/store'

// Cross-tab handoff. "Use" on a WARP endpoint pushes it here and switches to the
// Replacer's WireGuard mode; the IP-scanner / replacer push proxy endpoints via
// pendingProxyEndpoints. The Replacer component consumes these.
export const pendingWarpEndpoint = writable(null)
export const pendingProxyEndpoints = writable(null)
export const replacerCtype = writable('proxy')
