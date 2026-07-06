/*
 * TruthChain SDK — drop-in client for websites.
 *
 * Load the crypto core first, then this file:
 *   <script src="truthchain-wallet.js"></script>
 *   <script src="truthchain-sdk.js"></script>
 *   <script>
 *     const tc = TruthChain.connect('https://node.example.com:8080');
 *     const w = tc.createWallet();          // key generated IN THE BROWSER
 *     await tc.requestFaucet(w.address);     // site funds the user (if enabled)
 *     await tc.post(w.privateKeyHex, 'hello, permanent world');
 *     const bal = await tc.getBalance(w.address);
 *   </script>
 *
 * Non-custodial: private keys are generated and used entirely client-side and
 * are never sent to the node. The node only receives already-signed objects.
 */
(function (root, factory) {
  const wallet =
    (typeof module !== 'undefined' && module.exports)
      ? require('./truthchain-wallet.js')
      : root.TruthChainWallet;
  const mod = factory(wallet);
  if (typeof module !== 'undefined' && module.exports) module.exports = mod;
  else root.TruthChain = mod;
})(typeof self !== 'undefined' ? self : this, function (Wallet) {
  'use strict';

  if (!Wallet) {
    throw new Error('TruthChain SDK: load truthchain-wallet.js before truthchain-sdk.js');
  }

  const fetchImpl =
    (typeof fetch !== 'undefined')
      ? fetch
      : (typeof require !== 'undefined' ? require('node-fetch') : null);

  function Client(nodeURL) {
    this.nodeURL = String(nodeURL).replace(/\/+$/, '');
  }

  Client.prototype._request = async function (method, path, body) {
    const opts = { method: method, headers: {} };
    if (body !== undefined) {
      opts.headers['Content-Type'] = 'application/json';
      opts.body = JSON.stringify(body);
    }
    const res = await fetchImpl(this.nodeURL + path, opts);
    const text = await res.text();
    let data;
    try { data = text ? JSON.parse(text) : null; } catch (e) { data = text; }
    if (!res.ok) {
      const msg = (data && data.error) || (typeof data === 'string' && data) || ('HTTP ' + res.status);
      const err = new Error(msg);
      err.status = res.status;
      err.body = data;
      throw err;
    }
    return data;
  };

  // ---- wallet (client-side, no network) ----
  Client.prototype.createWallet = function () { return Wallet.createWallet(); };
  Client.prototype.importWallet = function (privHex) { return Wallet.walletFromPrivateKey(privHex); };

  // ---- reads ----
  Client.prototype.getStatus = function () { return this._request('GET', '/status'); };
  Client.prototype.getBalance = async function (address) {
    const w = await this._request('GET', '/wallets/' + encodeURIComponent(address) + '/balance');
    return w.balance;
  };
  Client.prototype.getWallet = function (address) {
    return this._request('GET', '/wallets/' + encodeURIComponent(address));
  };
  Client.prototype.getLatestBlock = function () { return this._request('GET', '/blockchain/latest'); };
  Client.prototype.getChainLength = function () { return this._request('GET', '/blockchain/length'); };

  // ---- writes (signed client-side, node only relays) ----
  Client.prototype.post = function (privHex, content, timestamp) {
    const signed = Wallet.signPost(privHex, content, timestamp);
    return this._request('POST', '/posts', signed);
  };

  // transfer looks up the sender's current nonce and uses nonce+1 unless one is
  // supplied explicitly.
  Client.prototype.transfer = async function (privHex, to, amount, opts) {
    opts = opts || {};
    const from = Wallet.walletFromPrivateKey(privHex).address;
    let nonce = opts.nonce;
    if (nonce == null) {
      const info = await this.getWallet(from);
      nonce = (info && info.nonce != null ? info.nonce : 0) + 1;
    }
    const signed = Wallet.signTransfer(privHex, to, amount, nonce, opts.timestamp);
    return this._request('POST', '/transfers', signed);
  };

  // ---- distribution ----
  Client.prototype.requestFaucet = function (address) {
    return this._request('POST', '/faucet', { address: address });
  };

  return {
    connect: function (nodeURL) { return new Client(nodeURL); },
    Client: Client,
  };
});
