export function createMediaQuery(query) {
  let matches = $state(false);

  if (typeof window !== 'undefined') {
    const media = window.matchMedia(query);
    matches = media.matches;
    media.addEventListener('change', (e) => matches = e.matches);
  }

  return { get matches() { return matches; } };
}
