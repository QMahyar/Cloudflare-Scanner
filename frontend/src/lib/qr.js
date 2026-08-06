import QRCode from 'qrcode'

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
