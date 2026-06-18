import { showToast } from "./toast.js";

// Shared AudioContext — browsers cap concurrent instances (Chrome: 6). Lazily
// created on first beep so it's never allocated on pages that don't use audio.
let _ac = null;
function getAC() {
	if (_ac && _ac.state !== "closed") return _ac;
	const Ctx = window.AudioContext || window["webkitAudioContext"];
	if (!Ctx) return null;
	_ac = new Ctx();
	return _ac;
}

// Short two-tone beep via WebAudio, matching the original completion chime.
function beep() {
	try {
		const ac = getAC();
		if (!ac) return;
		if (ac.state === "suspended") ac.resume();
		const o = ac.createOscillator();
		const g = ac.createGain();
		o.type = "sine";
		o.frequency.value = 880;
		g.gain.value = 0.08;
		o.connect(g);
		g.connect(ac.destination);
		o.start();
		o.frequency.setValueAtTime(660, ac.currentTime + 0.12);
		g.gain.setValueAtTime(0.08, ac.currentTime + 0.22);
		g.gain.linearRampToValueAtTime(0, ac.currentTime + 0.3);
		o.stop(ac.currentTime + 0.32);
	} catch {}
}

// notifyDone: in-page toast (always) + desktop notification (best-effort, asks
// permission lazily) + beep.
export function notifyDone(title, body) {
	showToast(body);
	beep();
	try {
		if (!("Notification" in window)) return;
		const fire = () => {
			try {
				new Notification(title, { body });
			} catch {}
		};
		if (Notification.permission === "granted") fire();
		else if (Notification.permission !== "denied") {
			Notification.requestPermission().then((p) => {
				if (p === "granted") fire();
			});
		}
	} catch {}
}

// " · 320/s · ~12s left" — live throughput + ETA. tEta formats the ETA string.
export function scanRateText(progress, total, startMs, tEta) {
	if (!startMs || progress <= 0) return "";
	const elapsed = (Date.now() - startMs) / 1000;
	if (elapsed < 0.6) return "";
	const rate = progress / elapsed;
	if (rate <= 0) return "";
	let s = " · " + (rate >= 10 ? Math.round(rate) : rate.toFixed(1)) + "/s";
	if (total > progress)
		s += " · " + tEta(Math.max(1, Math.round((total - progress) / rate)));
	return s;
}
