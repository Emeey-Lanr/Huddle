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

// Large text needs 3:1 against the page. A seller's pale colour falls back to
// near-black for text, and still shows up in the tiles.
export function readableInk(hex: string) {
  return (0.95 + 0.05) / (luminance(hex) + 0.05) >= 3 ? hex : "#16130f";
}

// Text placed on a filled button of this colour.
export const onColor = (hex: string) => (luminance(hex) > 0.4 ? "#16130f" : "#ffffff");
