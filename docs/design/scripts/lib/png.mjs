// png.mjs — a dependency-free PNG decoder and encoder.
//
// Why this exists rather than `npm i pngjs`: cst-0004 says the toolchain must
// work with the network unplugged, and DESIGN.md already pays for one exception
// (Playwright, which lives in the session scratchpad and is gitignored). A pixel
// gate that needs a second registry fetch to run is a gate that stops running the
// first time someone clones the repo on a plane. Node ships zlib; PNG is zlib
// plus five line filters plus a CRC. That is the whole cost.
//
// Scope, stated so a caller is never silently wrong:
//   - bit depth 8 only. 16-bit input throws rather than being quietly truncated,
//     because truncating is how a real 1-bit-per-channel drift becomes invisible.
//   - no Adam7 interlace. Playwright never emits it; if something else does, this
//     throws instead of decoding garbage.
//   - colour types 0 (grey), 2 (RGB), 3 (palette), 4 (grey+alpha), 6 (RGBA) all
//     decode to straight RGBA.

import { inflateSync, deflateSync } from 'node:zlib';

const SIG = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);
const CHANNELS = { 0: 1, 2: 3, 3: 1, 4: 2, 6: 4 };

// ---- CRC32 (the PNG variant: reflected, poly 0xedb88320) --------------------
const CRC_TABLE = (() => {
  const t = new Uint32Array(256);
  for (let n = 0; n < 256; n++) {
    let c = n;
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    t[n] = c >>> 0;
  }
  return t;
})();

function crc32(buf) {
  let c = 0xffffffff;
  for (let i = 0; i < buf.length; i++) c = CRC_TABLE[(c ^ buf[i]) & 0xff] ^ (c >>> 8);
  return (c ^ 0xffffffff) >>> 0;
}

// ---- line filters ------------------------------------------------------------
const paeth = (a, b, c) => {
  const p = a + b - c;
  const pa = Math.abs(p - a), pb = Math.abs(p - b), pc = Math.abs(p - c);
  return pa <= pb && pa <= pc ? a : pb <= pc ? b : c;
};

/**
 * Decode a PNG buffer to { width, height, data } where data is RGBA, 4 bytes
 * per pixel, row-major, no padding.
 */
export function decodePNG(buf, label = '<png>') {
  if (buf.length < 8 || !buf.subarray(0, 8).equals(SIG)) {
    throw new Error(`${label}: not a PNG (bad signature)`);
  }

  let off = 8;
  let ihdr = null, palette = null, trns = null;
  const idat = [];

  while (off + 8 <= buf.length) {
    const len = buf.readUInt32BE(off);
    const type = buf.toString('latin1', off + 4, off + 8);
    const data = buf.subarray(off + 8, off + 8 + len);
    if (type === 'IHDR') {
      ihdr = {
        width: data.readUInt32BE(0), height: data.readUInt32BE(4),
        depth: data[8], color: data[9], interlace: data[12],
      };
    } else if (type === 'PLTE') palette = Buffer.from(data);
    else if (type === 'tRNS') trns = Buffer.from(data);
    else if (type === 'IDAT') idat.push(Buffer.from(data));
    else if (type === 'IEND') break;
    off += 12 + len;
  }

  if (!ihdr) throw new Error(`${label}: no IHDR chunk`);
  if (ihdr.depth !== 8) {
    throw new Error(`${label}: bit depth ${ihdr.depth} is unsupported — this decoder ` +
      `handles depth 8 only, and refuses to truncate rather than hide a real difference`);
  }
  if (ihdr.interlace !== 0) throw new Error(`${label}: Adam7 interlace is unsupported`);
  const ch = CHANNELS[ihdr.color];
  if (!ch) throw new Error(`${label}: unknown colour type ${ihdr.color}`);
  if (ihdr.color === 3 && !palette) throw new Error(`${label}: colour type 3 with no PLTE`);
  if (!idat.length) throw new Error(`${label}: no IDAT chunks`);

  const { width, height } = ihdr;
  const raw = inflateSync(Buffer.concat(idat));
  const stride = width * ch;
  const expected = (stride + 1) * height;
  if (raw.length < expected) {
    throw new Error(`${label}: truncated image data (${raw.length} of ${expected} bytes)`);
  }

  // ---- un-filter into a flat <ch>-per-pixel buffer ----
  const flat = Buffer.allocUnsafe(stride * height);
  let prev = Buffer.alloc(stride);
  for (let y = 0; y < height; y++) {
    const ft = raw[y * (stride + 1)];
    const line = raw.subarray(y * (stride + 1) + 1, y * (stride + 1) + 1 + stride);
    const cur = flat.subarray(y * stride, (y + 1) * stride);
    for (let i = 0; i < stride; i++) {
      const a = i >= ch ? cur[i - ch] : 0;
      const b = prev[i];
      const c = i >= ch ? prev[i - ch] : 0;
      const x = line[i];
      switch (ft) {
        case 0: cur[i] = x; break;
        case 1: cur[i] = (x + a) & 0xff; break;
        case 2: cur[i] = (x + b) & 0xff; break;
        case 3: cur[i] = (x + ((a + b) >> 1)) & 0xff; break;
        case 4: cur[i] = (x + paeth(a, b, c)) & 0xff; break;
        default: throw new Error(`${label}: unknown filter type ${ft} on row ${y}`);
      }
    }
    prev = cur;
  }

  // ---- expand to RGBA ----
  const out = Buffer.allocUnsafe(width * height * 4);
  for (let p = 0; p < width * height; p++) {
    const s = p * ch, d = p * 4;
    switch (ihdr.color) {
      case 0: out[d] = out[d + 1] = out[d + 2] = flat[s]; out[d + 3] = 255; break;
      case 2: out[d] = flat[s]; out[d + 1] = flat[s + 1]; out[d + 2] = flat[s + 2]; out[d + 3] = 255; break;
      case 3: {
        const i = flat[s] * 3;
        out[d] = palette[i]; out[d + 1] = palette[i + 1]; out[d + 2] = palette[i + 2];
        out[d + 3] = trns && flat[s] < trns.length ? trns[flat[s]] : 255;
        break;
      }
      case 4: out[d] = out[d + 1] = out[d + 2] = flat[s]; out[d + 3] = flat[s + 1]; break;
      case 6: out[d] = flat[s]; out[d + 1] = flat[s + 1]; out[d + 2] = flat[s + 2]; out[d + 3] = flat[s + 3]; break;
    }
  }
  return { width, height, data: out };
}

// ---- encode ------------------------------------------------------------------
function chunk(type, data) {
  const len = Buffer.allocUnsafe(4);
  len.writeUInt32BE(data.length, 0);
  const body = Buffer.concat([Buffer.from(type, 'latin1'), data]);
  const crc = Buffer.allocUnsafe(4);
  crc.writeUInt32BE(crc32(body), 0);
  return Buffer.concat([len, body, crc]);
}

/** Encode RGBA (4 bytes per pixel) to a PNG buffer. Filter 0 on every row. */
export function encodePNG({ width, height, data }) {
  const stride = width * 4;
  const raw = Buffer.allocUnsafe((stride + 1) * height);
  for (let y = 0; y < height; y++) {
    raw[y * (stride + 1)] = 0;
    data.copy(raw, y * (stride + 1) + 1, y * stride, (y + 1) * stride);
  }
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(width, 0);
  ihdr.writeUInt32BE(height, 4);
  ihdr[8] = 8; ihdr[9] = 6; ihdr[10] = 0; ihdr[11] = 0; ihdr[12] = 0;
  return Buffer.concat([
    SIG,
    chunk('IHDR', ihdr),
    chunk('IDAT', deflateSync(raw, { level: 6 })),
    chunk('IEND', Buffer.alloc(0)),
  ]);
}
