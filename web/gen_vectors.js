// Generates interop test vectors with the browser wallet crypto, for the Go
// side to verify. Run: node web/gen_vectors.js > chain/testdata/interop_vectors.json
const w = require('./truthchain-wallet.js');

const account = w.createWallet();
const recipient = w.createWallet();

const post = w.signPost(account.privateKeyHex, 'Hello, permanent world — TruthChain interop test.', 1700000000);
const transfer = w.signTransfer(account.privateKeyHex, recipient.address, 100, 1, 1700000000);

process.stdout.write(JSON.stringify({ account, recipient, post, transfer }, null, 2) + '\n');
