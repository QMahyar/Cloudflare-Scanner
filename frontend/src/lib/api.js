// apiJSON — fetch wrapper that parses JSON and throws on non-2xx with the
// server's {error} message. Faithful port of the original apiJSON().
export function csrfToken() {
	return (
		document.cookie
			.split(";")
			.map((v) => v.trim())
			.find((v) => v.startsWith("cfscanner_token="))
			?.slice("cfscanner_token=".length) || ""
	);
}

export function withCSRF(opts = {}) {
	const token = csrfToken();
	if (!token) return opts;
	const headers = new Headers(opts.headers || {});
	headers.set("X-CSRF-Token", token);
	return { ...opts, headers };
}

export async function apiJSON(url, opts = {}) {
	const resp = await fetch(url, withCSRF(opts));
	const data = await resp.json().catch(() => ({}));
	if (!resp.ok) throw new Error(data.error || resp.statusText);
	return data;
}
