import { readFileSync, existsSync } from "node:fs"
import path from "node:path"
import { describe, it, expect } from "vitest"
import sharp from "sharp"

/**
 * Locate the webapp package. The webapp test run starts in `webapp/`, while the
 * root focused-spec Vitest project starts in the repository root, so try both.
 */
function findWebappDir(): string {
  const candidates = [process.cwd(), path.resolve(process.cwd(), "webapp")]
  for (const candidate of candidates) {
    if (existsSync(path.join(candidate, "public", "manifest.json"))) return candidate
  }
  throw new Error("could not locate the webapp package directory")
}

const webappDir = findWebappDir()
const publicDir = path.join(webappDir, "public") + path.sep

function readPublic(fileName: string): Buffer {
  return readFileSync(`${publicDir}${fileName}`)
}

function readText(fileName: string): string {
  return readPublic(fileName).toString("utf8")
}

/** Read width/height from a PNG IHDR chunk. */
function pngSize(buffer: Buffer): { width: number; height: number } {
  expect(buffer.subarray(1, 4).toString("ascii")).toBe("PNG")
  return { width: buffer.readUInt32BE(16), height: buffer.readUInt32BE(20) }
}

interface ManifestIcon {
  src: string
  sizes: string
  type: string
  purpose?: string
}

interface Manifest {
  icons: ManifestIcon[]
}

const manifest = JSON.parse(readText("manifest.json")) as Manifest

function iconsWithoutMaskable(): ManifestIcon[] {
  return manifest.icons.filter((icon) => icon.purpose !== "maskable")
}

describe("branding assets", () => {
  it("serves the branded favicon referenced from the HTML head", () => {
    const html = readFileSync(path.join(webappDir, "index.html"), "utf8")

    expect(html).toMatch(/<link rel="icon" type="image\/png" href="\/favicon\.png" \/>/)
    expect(existsSync(`${publicDir}favicon.png`)).toBe(true)
    expect(pngSize(readPublic("favicon.png")).width).toBeGreaterThan(0)
  })

  it("precaches the favicon in the service worker", () => {
    const sw = readText("sw.js")
    const staticAssets = sw.slice(sw.indexOf("STATIC_ASSETS"), sw.indexOf("];"))

    expect(staticAssets).toContain("'/favicon.png'")
    expect(staticAssets).toContain("'/apple-touch-icon.png'")
    expect(staticAssets).toContain("'/manifest.json'")
  })

  it("lists every PWA icon size in the manifest with matching files", () => {
    const regularSizes = ["72x72", "96x96", "128x128", "144x144", "152x152", "192x192", "384x384", "512x512"]

    for (const size of regularSizes) {
      const [width, height] = size.split("x").map(Number)
      const icon = iconsWithoutMaskable().find((candidate) => candidate.sizes === size)
      expect(icon, `manifest is missing the ${size} icon`).toBeTruthy()
      expect(icon?.type).toBe("image/png")

      const file = pngSize(readPublic(icon!.src.replace(/^\//, "")))
      expect(file).toEqual({ width, height })
    }

    const appleTouchIcon = pngSize(readPublic("apple-touch-icon.png"))
    expect(appleTouchIcon.width).toBe(appleTouchIcon.height)
  })

  it("declares maskable icons and keeps the mark inside the safe zone", async () => {
    const maskable = manifest.icons.filter((icon) => icon.purpose === "maskable")
    expect(maskable.map((icon) => icon.sizes).sort()).toEqual(["192x192", "512x512"])

    for (const icon of maskable) {
      const file = pngSize(readPublic(icon.src.replace(/^\//, "")))
      const [size] = icon.sizes.split("x").map(Number)
      expect(file).toEqual({ width: size, height: size })

      // The safe zone is the central 80% of the canvas. Every pixel that differs
      // from the background must stay inside it, so no platform mask clips the mark.
      const { data, info } = await sharp(`${publicDir}${icon.src.replace(/^\//, "")}`)
        .ensureAlpha()
        .raw()
        .toBuffer({ resolveWithObject: true })

      const corner = [data[0], data[1], data[2], data[3]]
      const margin = Math.round(info.width * 0.1)
      let outsideMark = 0

      for (let y = 0; y < info.height; y++) {
        for (let x = 0; x < info.width; x++) {
          const offset = (y * info.width + x) * 4
          const differs =
            data[offset] !== corner[0] ||
            data[offset + 1] !== corner[1] ||
            data[offset + 2] !== corner[2] ||
            data[offset + 3] !== corner[3]
          if (!differs) continue
          const inSafeZone =
            x >= margin && x < info.width - margin && y >= margin && y < info.height - margin
          if (!inSafeZone) outsideMark++
        }
      }

      expect(outsideMark, `${icon.src} paints outside the maskable safe zone`).toBe(0)
    }
  })

  it("ships the branded logo assets used by the auth screen and sidebar", () => {
    const welcome = pngSize(readPublic("flashbacks_logo_welcomescreen.png"))
    const base = pngSize(readPublic("flashbacks_logo_base.png"))

    expect(welcome.width).toBeGreaterThan(0)
    expect(base.width).toBeGreaterThan(0)
    expect(base.width).toBeGreaterThan(base.height)
  })
})
