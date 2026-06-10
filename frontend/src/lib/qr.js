import QRCode from 'qrcode'

// Generate a QR code as a data: URL, matching the original's dark-theme colors
// (light modules on a near-black background) and ~220px size. Offline + CSP-safe
// (data: is allowed by img-src). Returns '' on failure so callers can fall back.
export async function makeQRDataURL(text) {
  try {
    return await QRCode.toDataURL(text, {
      width: 220,
      margin: 1,
      errorCorrectionLevel: 'M',
      color: { dark: '#e6edf3', light: '#0d1117' },
    })
  } catch {
    return ''
  }
}
