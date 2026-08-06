export function activateKey(fn) {
	return (node) => {

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
