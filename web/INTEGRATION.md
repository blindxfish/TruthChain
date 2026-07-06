# Embedding TruthChain in your website

TruthChain can be embedded into any website so your users can create wallets,
post to the permanent record, and receive/spend characters — **without running a
node themselves and without your site ever touching their private keys.**

Three pieces make this work:

1. **A node** you run (the gateway your users' browsers talk to).
2. **The JS SDK** (`truthchain-wallet.js` + `truthchain-sdk.js`) you drop into your page.
3. **A faucet** on your node that hands characters to new users so they can post.

Private keys are generated **in the user's browser** and are used to sign posts
and transfers **client-side**. The node only ever receives already-signed
objects — it cannot spend a user's balance or impersonate them. This is the same
non-custodial model as a browser crypto wallet.

---

## 1. Run a node (headless)

The node runs non-interactively from environment variables — ideal for a server
or Docker.

```bash
# build
go build -o truthchain_server cmd/main.go

# run a public gateway with a faucet
TRUTHCHAIN_NETWORK=testnet \
TRUTHCHAIN_PUBLIC_API=true \
TRUTHCHAIN_API_RATE_LIMIT=120 \
TRUTHCHAIN_FAUCET=true \
TRUTHCHAIN_FAUCET_AMOUNT=100 \
TRUTHCHAIN_FAUCET_COOLDOWN_SEC=3600 \
TRUTHCHAIN_FAUCET_DAILY_CAP=100000 \
./truthchain_server --headless
```

Environment variables (all optional):

| Var | Default | Meaning |
|---|---|---|
| `TRUTHCHAIN_NETWORK` | `local` | `local` / `testnet` / `mainnet` |
| `TRUTHCHAIN_API_PORT` | `8080` | HTTP API port |
| `TRUTHCHAIN_MESH_PORT` | `9876` | P2P mesh port |
| `TRUTHCHAIN_MODES` | `api,mesh,mining` | comma list: `api,mesh,mining,beacon` |
| `TRUTHCHAIN_PUBLIC_API` | `false` | bind API to `0.0.0.0` (reachable off-host) |
| `TRUTHCHAIN_API_RATE_LIMIT` | `120` | requests/min per IP |
| `TRUTHCHAIN_FAUCET` | `false` | enable `POST /faucet` |
| `TRUTHCHAIN_FAUCET_AMOUNT` | `100` | characters per claim |
| `TRUTHCHAIN_FAUCET_COOLDOWN_SEC` | `3600` | per-address cooldown |
| `TRUTHCHAIN_FAUCET_DAILY_CAP` | `10000` | max characters dispensed/day |

**Security notes**

- The public API exposes only reads and **client-signed** submits. Key-bearing
  endpoints (`/local/*`, wallet backup) stay loopback-only and are refused for
  remote callers.
- The faucet spends **your node's** character balance (earned by uptime mining
  or received via transfer). Fund it and set the caps to control your exposure.
- Put the node behind TLS (a reverse proxy) so browsers use `https://`.

---

## 2. Add the SDK to your page

```html
<script src="truthchain-wallet.js"></script>
<script src="truthchain-sdk.js"></script>
<script>
  const tc = TruthChain.connect('https://node.example.com:8080');

  // Wallet is generated IN THE BROWSER — the key never leaves the device.
  const wallet = tc.createWallet();          // { address, privateKeyHex, publicKeyHex }
  localStorage.setItem('tc_key', wallet.privateKeyHex); // your app decides storage

  await tc.requestFaucet(wallet.address);     // ask your node to fund the user
  await tc.post(wallet.privateKeyHex, 'hello, permanent world');
  const balance = await tc.getBalance(wallet.address);
  await tc.transfer(wallet.privateKeyHex, someAddress, 10); // nonce fetched automatically
</script>
```

See [`demo.html`](demo.html) for a complete working page. Serve the three files
together and point the URL field at your node.

### SDK API

| Call | Network? | Description |
|---|---|---|
| `tc.createWallet()` | no | generate a keypair + address in the browser |
| `tc.importWallet(privHex)` | no | rebuild a wallet from a saved key |
| `tc.getStatus()` | GET | node/chain status |
| `tc.getBalance(addr)` | GET | character balance |
| `tc.getWallet(addr)` | GET | `{ address, balance, nonce }` |
| `tc.post(privHex, content)` | POST | sign client-side, submit to `/posts` |
| `tc.transfer(privHex, to, amount)` | POST | sign client-side, submit to `/transfers` |
| `tc.requestFaucet(addr)` | POST | ask the node to send the user characters |

---

## 3. How characters flow

1. A visitor creates a wallet in their browser (0 characters).
2. Your site calls `requestFaucet(address)` → your node signs a transfer of
   characters from its own wallet to the user (subject to cooldown/cap).
3. Once that transfer is in a block, the user can `post()` (posting burns
   characters equal to the message length) or `transfer()` to others.
4. Any node answers `getBalance()` from its copy of the chain — balances are
   derived from block state, not stored per-wallet, and are **never synced as
   wallets**; only the chain syncs.

---

## Interop guarantee

The browser crypto is byte-compatible with the Go node: addresses are Base58Check
(version `0x00`), signatures are compact-recoverable secp256k1 with the
compressed-key flag, low-S normalized. This is enforced by a cross-language test
(`chain/wallet_interop_test.go`) that verifies browser-produced signatures with
the real node code.
