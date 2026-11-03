export function price(amount: string, currency: string) {
  return new Intl.NumberFormat("en-NG", {
    style: "currency",
    currency,
    maximumFractionDigits: Number(amount) % 1 === 0 ? 0 : 2,
  }).format(Number(amount));
}

// Asks Cloudinary for a modern format at the size needed, so a phone on mobile
// data never downloads the seller's full-size photo. Other links pass through.
export function photo(url: string, width: number) {
  return url.includes("res.cloudinary.com")
    ? url.replace("/upload/", `/upload/f_auto,q_auto,c_limit,w_${width}/`)
    : url;
}

export function photoSet(url: string, widths: number[]) {
  if (!url.includes("res.cloudinary.com")) return undefined;
  return widths.map((w) => `${photo(url, w)} ${w}w`).join(", ");
}

function luminance(hex: string) {
  const [r, g, b] = [1, 3, 5]
    .map((i) => parseInt(hex.slice(i, i + 2), 16) / 255)
    .map((c) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4));
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

const contrast = (a: number, b: number) => (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);

const nearBlack = "#111111";

// Text on a page flooded with this colour: whichever of black or white reads
// better, which always clears 4.5:1, so any colour a seller picks stays legible.
export function onColor(hex: string) {
  const ground = luminance(hex);
  return contrast(ground, 1) >= contrast(ground, luminance(nearBlack)) ? "#ffffff" : nearBlack;
}

// Text on the white bag and checkout pages: the store's colour if it reads well
// there, otherwise near-black.
export const inkOnWhite = (hex: string) => (contrast(luminance(hex), 1) >= 4.5 ? hex : nearBlack);
