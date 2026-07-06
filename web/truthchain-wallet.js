/*
 * TruthChain browser wallet crypto — self-contained, no dependencies.
 *
 * Produces keys, addresses and signatures that interoperate byte-for-byte with
 * the Go node (wallet.DeriveAddress, VerifyPostSignature, Transfer.VerifySignature).
 * Runs in the browser and in Node.js (used by gen_vectors.js for cross-testing).
 *
 * Key facts it must match:
 *  - secp256k1, compressed public keys, Base58Check address (version byte 0x00),
 *    checksum = first 4 bytes of double-SHA256.
 *  - Signatures are 65-byte COMPACT RECOVERABLE sigs: [27 + recid + 4][R:32][S:32]
 *    (the +4 marks a compressed key, which the node requires), low-S normalized.
 *  - Post digest  = SHA256(author + content + timestamp)
 *  - Transfer sig digest = SHA256("from:to:amount:gas_fee:timestamp:nonce")
 *  - Transfer hash = SHA256(Go-json{amount,from,gas_fee,nonce,timestamp,to})  // keys sorted
 */
(function (root, factory) {
  const mod = factory();
  if (typeof module !== 'undefined' && module.exports) module.exports = mod;
  else root.TruthChainWallet = mod;
})(typeof self !== 'undefined' ? self : this, function () {
  'use strict';

  // ---------- byte / hex helpers ----------
  function bytesToHex(b) {
    let s = '';
    for (let i = 0; i < b.length; i++) s += b[i].toString(16).padStart(2, '0');
    return s;
  }
  function hexToBytes(h) {
    const b = new Uint8Array(h.length / 2);
    for (let i = 0; i < b.length; i++) b[i] = parseInt(h.substr(i * 2, 2), 16);
    return b;
  }
  function utf8ToBytes(str) {
    if (typeof TextEncoder !== 'undefined') return new TextEncoder().encode(str);
    // Node fallback
    return Uint8Array.from(Buffer.from(str, 'utf8'));
  }
  function concatBytes() {
    let len = 0;
    for (const a of arguments) len += a.length;
    const out = new Uint8Array(len);
    let o = 0;
    for (const a of arguments) { out.set(a, o); o += a.length; }
    return out;
  }
  function randomBytes(n) {
    if (typeof crypto !== 'undefined' && crypto.getRandomValues) {
      const b = new Uint8Array(n);
      crypto.getRandomValues(b);
      return b;
    }
    // Node
    return Uint8Array.from(require('crypto').randomBytes(n));
  }

  // ---------- SHA-256 ----------
  function sha256(msg) {
    const K = [
      0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
      0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
      0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
      0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
      0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
      0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
      0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
      0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2];
    let h0 = 0x6a09e667, h1 = 0xbb67ae85, h2 = 0x3c6ef372, h3 = 0xa54ff53a,
        h4 = 0x510e527f, h5 = 0x9b05688c, h6 = 0x1f83d9ab, h7 = 0x5be0cd19;
    const l = msg.length;
    const withPad = new Uint8Array((((l + 8) >> 6) + 1) << 6);
    withPad.set(msg);
    withPad[l] = 0x80;
    const bitLen = l * 8;
    const dv = new DataView(withPad.buffer);
    dv.setUint32(withPad.length - 4, bitLen >>> 0);
    dv.setUint32(withPad.length - 8, Math.floor(bitLen / 0x100000000));
    const w = new Uint32Array(64);
    const rotr = (x, n) => (x >>> n) | (x << (32 - n));
    for (let off = 0; off < withPad.length; off += 64) {
      for (let i = 0; i < 16; i++) w[i] = dv.getUint32(off + i * 4);
      for (let i = 16; i < 64; i++) {
        const s0 = rotr(w[i - 15], 7) ^ rotr(w[i - 15], 18) ^ (w[i - 15] >>> 3);
        const s1 = rotr(w[i - 2], 17) ^ rotr(w[i - 2], 19) ^ (w[i - 2] >>> 10);
        w[i] = (w[i - 16] + s0 + w[i - 7] + s1) | 0;
      }
      let a = h0, b = h1, c = h2, d = h3, e = h4, f = h5, g = h6, h = h7;
      for (let i = 0; i < 64; i++) {
        const S1 = rotr(e, 6) ^ rotr(e, 11) ^ rotr(e, 25);
        const ch = (e & f) ^ (~e & g);
        const t1 = (h + S1 + ch + K[i] + w[i]) | 0;
        const S0 = rotr(a, 2) ^ rotr(a, 13) ^ rotr(a, 22);
        const maj = (a & b) ^ (a & c) ^ (b & c);
        const t2 = (S0 + maj) | 0;
        h = g; g = f; f = e; e = (d + t1) | 0; d = c; c = b; b = a; a = (t1 + t2) | 0;
      }
      h0 = (h0 + a) | 0; h1 = (h1 + b) | 0; h2 = (h2 + c) | 0; h3 = (h3 + d) | 0;
      h4 = (h4 + e) | 0; h5 = (h5 + f) | 0; h6 = (h6 + g) | 0; h7 = (h7 + h) | 0;
    }
    const out = new Uint8Array(32);
    const ov = new DataView(out.buffer);
    [h0, h1, h2, h3, h4, h5, h6, h7].forEach((v, i) => ov.setUint32(i * 4, v >>> 0));
    return out;
  }

  // ---------- RIPEMD-160 ----------
  function ripemd160(msg) {
    const rol = (x, n) => ((x << n) | (x >>> (32 - n))) >>> 0;
    const zl = [0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,7,4,13,1,10,6,15,3,12,0,9,5,2,14,11,8,
      3,10,14,4,9,15,8,1,2,7,0,6,13,11,5,12,1,9,11,10,0,8,12,4,13,3,7,15,14,5,6,2,
      4,0,5,9,7,12,2,10,14,1,3,8,11,6,15,13];
    const zr = [5,14,7,0,9,2,11,4,13,6,15,8,1,10,3,12,6,11,3,7,0,13,5,10,14,15,8,12,4,9,1,2,
      15,5,1,3,7,14,6,9,11,8,12,2,10,0,4,13,8,6,4,1,3,11,15,0,5,12,2,13,9,7,10,14,
      12,15,10,4,1,5,8,7,6,2,13,14,0,3,9,11];
    const sl = [11,14,15,12,5,8,7,9,11,13,14,15,6,7,9,8,7,6,8,13,11,9,7,15,7,12,15,9,11,7,13,12,
      11,13,6,7,14,9,13,15,14,8,13,6,5,12,7,5,11,12,14,15,14,15,9,8,9,14,5,6,8,6,5,12,
      9,15,5,11,6,8,13,12,5,12,13,14,11,8,5,6];
    const sr = [8,9,9,11,13,15,15,5,7,7,8,11,14,14,12,6,9,13,15,7,12,8,9,11,7,7,12,7,6,15,13,11,
      9,7,15,11,8,6,6,14,12,13,5,14,13,13,7,5,15,5,8,11,14,14,6,14,6,9,12,9,12,5,15,8,
      8,5,12,9,12,5,14,6,8,13,6,5,15,13,11,11];
    const hl = [0, 0x5a827999, 0x6ed9eba1, 0x8f1bbcdc, 0xa953fd4e];
    const hr = [0x50a28be6, 0x5c4dd124, 0x6d703ef3, 0x7a6d76e9, 0];
    const f = (j, x, y, z) => {
      if (j < 16) return (x ^ y ^ z);
      if (j < 32) return ((x & y) | (~x & z));
      if (j < 48) return ((x | ~y) ^ z);
      if (j < 64) return ((x & z) | (y & ~z));
      return (x ^ (y | ~z));
    };
    let h0 = 0x67452301, h1 = 0xefcdab89, h2 = 0x98badcfe, h3 = 0x10325476, h4 = 0xc3d2e1f0;
    const l = msg.length;
    const withPad = new Uint8Array((((l + 8) >> 6) + 1) << 6);
    withPad.set(msg);
    withPad[l] = 0x80;
    const dv = new DataView(withPad.buffer);
    const bitLen = l * 8;
    dv.setUint32(withPad.length - 8, bitLen >>> 0, true);
    dv.setUint32(withPad.length - 4, Math.floor(bitLen / 0x100000000), true);
    const X = new Uint32Array(16);
    for (let off = 0; off < withPad.length; off += 64) {
      for (let i = 0; i < 16; i++) X[i] = dv.getUint32(off + i * 4, true);
      let al = h0, bl = h1, cl = h2, dl = h3, el = h4;
      let ar = h0, br = h1, cr = h2, dr = h3, er = h4;
      for (let j = 0; j < 80; j++) {
        let t = (al + f(j, bl, cl, dl) + X[zl[j]] + hl[(j / 16) | 0]) | 0;
        t = (rol(t >>> 0, sl[j]) + el) | 0;
        al = el; el = dl; dl = rol(cl, 10); cl = bl; bl = t;
        t = (ar + f(79 - j, br, cr, dr) + X[zr[j]] + hr[(j / 16) | 0]) | 0;
        t = (rol(t >>> 0, sr[j]) + er) | 0;
        ar = er; er = dr; dr = rol(cr, 10); cr = br; br = t;
      }
      const t = (h1 + cl + dr) | 0;
      h1 = (h2 + dl + er) | 0;
      h2 = (h3 + el + ar) | 0;
      h3 = (h4 + al + br) | 0;
      h4 = (h0 + bl + cr) | 0;
      h0 = t;
    }
    const out = new Uint8Array(20);
    const ov = new DataView(out.buffer);
    [h0, h1, h2, h3, h4].forEach((v, i) => ov.setUint32(i * 4, v >>> 0, true));
    return out;
  }

  // ---------- secp256k1 ----------
  const P = 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2Fn;
  const N = 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141n;
  const Gx = 0x79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798n;
  const Gy = 0x483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8n;

  function mod(a, m) { const r = a % m; return r >= 0n ? r : r + m; }
  function modinv(a, m) {
    let [old_r, r] = [mod(a, m), m];
    let [old_s, s] = [1n, 0n];
    while (r !== 0n) {
      const q = old_r / r;
      [old_r, r] = [r, old_r - q * r];
      [old_s, s] = [s, old_s - q * s];
    }
    return mod(old_s, m);
  }
  // Jacobian-free affine point ops (fine for a wallet's occasional signing).
  function pointDouble(pt) {
    if (pt === null) return null;
    const [x, y] = pt;
    if (y === 0n) return null;
    const s = mod((3n * x * x) * modinv(2n * y, P), P);
    const rx = mod(s * s - 2n * x, P);
    const ry = mod(s * (x - rx) - y, P);
    return [rx, ry];
  }
  function pointAdd(a, b) {
    if (a === null) return b;
    if (b === null) return a;
    const [x1, y1] = a, [x2, y2] = b;
    if (x1 === x2 && mod(y1 + y2, P) === 0n) return null;
    let s;
    if (x1 === x2 && y1 === y2) return pointDouble(a);
    s = mod((y2 - y1) * modinv(mod(x2 - x1, P), P), P);
    const rx = mod(s * s - x1 - x2, P);
    const ry = mod(s * (x1 - rx) - y1, P);
    return [rx, ry];
  }
  function scalarMul(k, pt) {
    let result = null;
    let addend = pt;
    while (k > 0n) {
      if (k & 1n) result = pointAdd(result, addend);
      addend = pointDouble(addend);
      k >>= 1n;
    }
    return result;
  }

  function bigToBytes32(v) {
    const out = new Uint8Array(32);
    for (let i = 31; i >= 0; i--) { out[i] = Number(v & 0xffn); v >>= 8n; }
    return out;
  }
  function bytesToBig(b) {
    let v = 0n;
    for (let i = 0; i < b.length; i++) v = (v << 8n) | BigInt(b[i]);
    return v;
  }

  function compressPub(pt) {
    const [x, y] = pt;
    const out = new Uint8Array(33);
    out[0] = (y & 1n) === 0n ? 0x02 : 0x03;
    out.set(bigToBytes32(x), 1);
    return out;
  }

  // Sign a 32-byte digest, returning a 65-byte compact recoverable signature
  // matching btcec's SignCompact(..., isCompressed=true).
  function signRecoverable(digest, privBig) {
    const z = mod(bytesToBig(digest), N);
    for (let attempt = 0; attempt < 64; attempt++) {
      let k = mod(bytesToBig(randomBytes(32)), N);
      if (k === 0n) continue;
      const R = scalarMul(k, [Gx, Gy]);
      const r = mod(R[0], N);
      if (r === 0n) continue;
      let s = mod(modinv(k, N) * (z + r * privBig), N);
      if (s === 0n) continue;
      let recid = (R[1] & 1n ? 1 : 0) | (R[0] >= N ? 2 : 0);
      // Low-S normalization (btcec produces canonical low-S).
      if (s > N >> 1n) { s = N - s; recid ^= 1; }
      const out = new Uint8Array(65);
      out[0] = 27 + recid + 4; // +4: signed by a compressed key
      out.set(bigToBytes32(r), 1);
      out.set(bigToBytes32(s), 33);
      return out;
    }
    throw new Error('failed to produce signature');
  }

  // ---------- Base58 / address ----------
  const B58 = '123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz';
  function base58encode(bytes) {
    let x = bytesToBig(bytes);
    let out = '';
    while (x > 0n) { const r = x % 58n; x = x / 58n; out = B58[Number(r)] + out; }
    for (let i = 0; i < bytes.length && bytes[i] === 0; i++) out = '1' + out;
    return out;
  }
  function base58check(payload) {
    const checksum = sha256(sha256(payload)).slice(0, 4);
    return base58encode(concatBytes(payload, checksum));
  }
  // DeriveAddress: version 0x00 + RIPEMD160(SHA256(compressedPub)) + checksum.
  function pubToAddress(compressedPub) {
    const h = ripemd160(sha256(compressedPub));
    const payload = concatBytes(new Uint8Array([0x00]), h);
    return base58check(payload);
  }

  // ---------- public API ----------
  function createWallet() {
    let priv;
    do { priv = mod(bytesToBig(randomBytes(32)), N); } while (priv === 0n);
    const pub = compressPub(scalarMul(priv, [Gx, Gy]));
    return {
      privateKeyHex: bytesToHex(bigToBytes32(priv)),
      publicKeyHex: bytesToHex(pub),
      address: pubToAddress(pub),
    };
  }

  function walletFromPrivateKey(hex) {
    const priv = bytesToBig(hexToBytes(hex));
    if (priv <= 0n || priv >= N) throw new Error('invalid private key');
    const pub = compressPub(scalarMul(priv, [Gx, Gy]));
    return { privateKeyHex: hex, publicKeyHex: bytesToHex(pub), address: pubToAddress(pub) };
  }

  // Build a signed post ready for POST /posts.
  function signPost(privHex, content, timestamp) {
    const w = walletFromPrivateKey(privHex);
    const ts = timestamp != null ? timestamp : Math.floor(Date.now() / 1000);
    const digestInput = utf8ToBytes(w.address + content + ts);
    const digest = sha256(digestInput);
    const sig = signRecoverable(digest, bytesToBig(hexToBytes(privHex)));
    return {
      author: w.address,
      content: content,
      timestamp: ts,
      hash: bytesToHex(digest), // Post.CalculateHash == SHA256(author+content+timestamp)
      signature: bytesToHex(sig),
    };
  }

  // Build a signed transfer ready for POST /transfers.
  function signTransfer(privHex, to, amount, nonce, timestamp) {
    const w = walletFromPrivateKey(privHex);
    const ts = timestamp != null ? timestamp : Math.floor(Date.now() / 1000);
    const gasFee = 1;
    // Hash must match Go json.Marshal of a map (keys sorted alphabetically).
    const hashJson =
      '{"amount":' + amount + ',"from":"' + w.address + '","gas_fee":' + gasFee +
      ',"nonce":' + nonce + ',"timestamp":' + ts + ',"to":"' + to + '"}';
    const hash = sha256(utf8ToBytes(hashJson));
    // Signature digest is over the colon-delimited form.
    const sigInput = w.address + ':' + to + ':' + amount + ':' + gasFee + ':' + ts + ':' + nonce;
    const sig = signRecoverable(sha256(utf8ToBytes(sigInput)), bytesToBig(hexToBytes(privHex)));
    return {
      from: w.address,
      to: to,
      amount: amount,
      gas_fee: gasFee,
      timestamp: ts,
      nonce: nonce,
      hash: bytesToHex(hash),
      signature: bytesToHex(sig),
    };
  }

  return {
    createWallet,
    walletFromPrivateKey,
    signPost,
    signTransfer,
    // low-level, exposed for testing
    _sha256: sha256,
    _ripemd160: ripemd160,
    _pubToAddress: pubToAddress,
  };
});
