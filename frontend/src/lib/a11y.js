// activateKey triggers `fn` on Enter/Space for non-button elements with role="button".
export function activateKey(node, fn) {
	let current = fn;
	function handler(e) {
		if (e.key === "Enter" || e.key === " ") {
			e.preventDefault();
			current(e);
		}
	}
	node.addEventListener("keydown", handler);
	return {
		update(newFn) {
			current = newFn;
		},
		destroy() {
			node.removeEventListener("keydown", handler);
		},
	};
}
