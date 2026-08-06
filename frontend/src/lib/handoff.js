import { writable } from 'svelte/store'

export const pendingWarpEndpoint = writable(null)
export const pendingProxyEndpoints = writable(null)
export const replacerCtype = writable('proxy')
