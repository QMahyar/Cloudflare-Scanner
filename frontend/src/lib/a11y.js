// activateKey returns a Svelte 5 attachment: on Enter/Space it runs `fn` for
// non-button elements with role="button". Prefer `{@attach activateKey(fn)}`
// over the legacy `use:` action form.
/**
 * @param {() => void | ((e: KeyboardEvent) => void)} fn
 * @returns {import('svelte/attachments').Attachment}
 */
export function activateKey(fn) {
	return (node) => {
		/** @param {KeyboardEvent} e */
		function handler(e) {
			if (e.key === 'Enter' || e.key === ' ') {
				e.preventDefault()
				fn(e)
			}
		}
		node.addEventListener('keydown', handler)
		return () => node.removeEventListener('keydown', handler)
	}
}
