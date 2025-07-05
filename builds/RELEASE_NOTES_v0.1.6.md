# TruthChain v0.1.6 Release Notes

## Highlights
- **Wallet-based handshake for mesh self-connection prevention**
  - Nodes now use wallet address in handshake to prevent self-connection, allowing safe use of domain names in bootstrap.json.
- **Domain-based bootstrap**
  - Release includes bootstrap.json with the mainnet domain, not a static IP, for easier node migration and discovery.
- **Improved periodic status logging**
  - Every minute, logs chain size, post count, characters minted, peer summary, and pending posts.
- **Reduced log spam**
  - Self-connection skips are now silent; only important network and blockchain events are logged.
- **Cleaner codebase and packaging**
  - Linter errors fixed, unnecessary code removed, and release artifacts include all required files.

## Files Included
- `truthchain-linux` (Linux binary)
- `truthchain.exe` (Windows binary)
- `bootstrap.json` (with domain)
- `README.md`

## Upgrade Notes
- Replace your old binary and bootstrap.json with the new versions.
- No need to update bootstrap.json for future mainnet node migrations—just update DNS.

---

Thank you for supporting TruthChain! 🚀 