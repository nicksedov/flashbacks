import sharp from "sharp";
import { readFileSync } from "fs";
import { resolve, dirname } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const sourcePath = resolve(__dirname, "../public/icon-highres.png");
const outDir = resolve(__dirname, "../public");

const BG_COLOR = { r: 0, g: 0, b: 0, alpha: 0 }; // transparent
const sourceBuffer = readFileSync(sourcePath);
const { width: sourceW, height: sourceH } = await sharp(sourceBuffer).metadata();

// Icon definitions: [name, size, isMaskable]
const icons = [
  ["icon-72.png", 72, false],
  ["icon-96.png", 96, false],
  ["icon-128.png", 128, false],
  ["icon-144.png", 144, false],
  ["icon-152.png", 152, false],
  ["icon-192.png", 192, false],
  ["icon-384.png", 384, false],
  ["icon-512.png", 512, false],
  ["apple-touch-icon.png", 180, false],
  ["favicon.png", 64, false],
  // Maskable icons with larger safe-zone padding
  ["maskable-192.png", 192, true],
  ["maskable-512.png", 512, true],
];

async function generateIcon(name, size, isMaskable) {
  // Maskable: content fills ~75% (safe zone is 80% circle)
  // Regular: content fills ~90%
  const contentRatio = isMaskable ? 0.75 : 0.90;
  const contentSize = Math.round(size * contentRatio);

  // Fit source into contentSize x contentSize without clipping
  const scale = Math.min(contentSize / sourceW, contentSize / sourceH);
  const renderW = Math.round(sourceW * scale);
  const renderH = Math.round(sourceH * scale);

  // Render source at target size
  const sourceRendered = await sharp(sourceBuffer)
    .resize(renderW, renderH, { fit: "contain", background: { r: 0, g: 0, b: 0, alpha: 0 } })
    .png()
    .toBuffer();

  // Center on square canvas with background color
  await sharp({
    create: {
      width: size,
      height: size,
      channels: 4,
      background: BG_COLOR,
    },
  })
    .composite([{
      input: sourceRendered,
      left: Math.round((size - renderW) / 2),
      top: Math.round((size - renderH) / 2),
    }])
    .png()
    .toFile(resolve(outDir, name));

  console.log(`  ${name}: ${size}x${size} (source at ${renderW}x${renderH})`);
}

console.log("Generating PWA icons...\n");

for (const [name, size, maskable] of icons) {
  await generateIcon(name, size, maskable);
}

console.log("\nDone! Icons generated in public/");
