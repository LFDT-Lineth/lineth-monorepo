# Dev keys inventory — DO NOT REUSE

> Every private key listed in the committed section is public knowledge. These
> keys exist only to make the local quickstart convenient. Never reuse, copy, or
> derive from them for any non-loopback network.

The quickstart now has three key classes:

- **Sepolia deployer keystore** — default Sepolia boot generates an encrypted
  ethers keystore under gitignored `artifacts/accounts/deployer-keystore/`.
  This account must be funded by the tester and is preserved by
  `./scripts/reset.sh` unless `--forget-deployer` is used.
- **Generated runtime keys** — `account-setup` creates L1 blob/data-submission,
  L1 finalization, L1 postman, L2 deployer, L2 anchorer, and L2 postman keys at
  first boot, then persists them under gitignored `artifacts/accounts/` for retries.
- **Committed local-dev keys** — static node identity and consensus keys that
  remain in the repo and are listed below.

Advanced users may point the Sepolia deployer resolver at an existing encrypted
keystore with `L1_DEPLOYER_KEYSTORE_PATH` plus `L1_DEPLOYER_KEYSTORE_PASSWORD`
or `L1_DEPLOYER_KEYSTORE_PASSWORD_FILE`. `L1_DEPLOYER_PRIVATE_KEY` remains only
as deprecated compatibility for existing setups and must not be used in the
default tester path or restored to `.env.example`.

## Committed local-dev keys

| Path | Format | Used by | Public knowledge? |
|------|--------|---------|-------------------|
| `config/services/sequencer/key` | raw hex SECP256K1 | Linea Besu sequencer node identity | yes |
| `config/services/maru/private-key` | raw hex SECP256K1 | Maru consensus signer | yes |

## TLS material (committed; mTLS only — not Sepolia-funds-controlling)

| Path | Format | Used by |
|------|--------|---------|
| `config/web3signer/tls-files/web3signer-keystore.p12` | PKCS#12 keystore | Web3signer mTLS server cert |
| `config/web3signer/tls-files/web3signer-keystore-password.txt` | plaintext password | Web3signer keystore password |
| `config/web3signer/tls-files/known-clients.txt` | client cert fingerprint list | Web3signer mTLS client allow-list |
| `config/services/coordinator/tls-files/coordinator-client-keystore.p12` | PKCS#12 keystore | Coordinator mTLS client cert |
| `config/services/postman/tls-files/postman-client-keystore.p12` | PKCS#12 keystore | Postman mTLS client cert |
| `config/services/postman/tls-files/web3signer-truststore.p12` | PKCS#12 truststore | Postman truststore for Web3signer mTLS |
| `config/services/sequencer/tls-files/sequencer_client_keystore.p12` | PKCS#12 keystore | Sequencer mTLS client cert |

## Runtime-generated keys (not committed)

`account-setup` writes these into the gitignored `artifacts/accounts` host directory:

- `/accounts/deployer-keystore/l1-deployer.json` — encrypted ethers JSON keystore
  for the generated Sepolia deployer
- `/accounts/deployer-keystore/password.txt` — local-dev password file for the
  generated Sepolia deployer keystore
- `/accounts/runtime-keys.env`
  - `L1_BLOB_SUBMITTER_PRIVATE_KEY`
  - `L1_FINALIZATION_SUBMITTER_PRIVATE_KEY`
  - `L1_POSTMAN_PRIVATE_KEY`
  - `L2_DEPLOYER_PRIVATE_KEY`
  - `L2_MESSAGE_ANCHORING_PRIVATE_KEY`
  - `L2_POSTMAN_PRIVATE_KEY`
- `/accounts/runtime-keystores/*.json` — encrypted ethers JSON keystores for generated runtime keys
- `/accounts/runtime-keystores/password.txt` — generated keystore password file for Web3Signer
- `/accounts/web3signer-keys/anchoring-signer.yaml` — Web3Signer `file-keystore` config for the generated L2 message anchoring key
- `/accounts/web3signer-keys/data-submission-signer.yaml` — Web3Signer `file-keystore` config for the generated L1 blob/data-submission key
- `/accounts/web3signer-keys/finalization-signer.yaml` — Web3Signer `file-keystore` config for the generated L1 aggregation/finalization key
- `/accounts/web3signer-keys/l1-postman-signer.yaml` — Web3Signer `file-keystore` config for the generated L1 postman key
- `/accounts/web3signer-keys/l2-postman-signer.yaml` — Web3Signer `file-keystore` config for the generated L2 postman key

`./scripts/reset.sh` wipes the generated runtime service keys; the next boot
generates a fresh runtime key set. It preserves
`/accounts/deployer-keystore/` by default because the generated Sepolia deployer
may hold tester-funded Sepolia ETH. Use `./scripts/reset.sh --forget-deployer`
only when intentionally deleting that deployer keystore and password.

Runtime service key artifacts are intentionally written with container-readable
host permissions. They are bind-mounted into Web3Signer, deploy, traffic, and
smoke-test containers that may not share the host user/group. This is acceptable
only for the gitignored dev/demo `artifacts/` directory; do not reuse this file
permission model for non-loopback or production key custody.

## Local genesis deployer

`L1_MODE=local` ignores Sepolia deployer keystore and raw-key config. It uses
the built-in local genesis deployer against the local L1 RPC defaults. That
local genesis private key is public test-only material and must not be reused
outside loopback/local-dev networks.

## Replacing keys for any non-loopback / non-Sepolia deployment

Before running this stack against anything other than Sepolia testnet:

1. Use a fresh, funded encrypted deployer keystore via `L1_DEPLOYER_KEYSTORE_PATH`
   and a password env/file. Do not use the generated demo deployer or the
   deprecated raw-key compatibility path for non-Sepolia environments.
2. Regenerate `config/services/sequencer/key` and update the l2-node-besu bootnode
   `enode://...` in `docker-compose.yml` to match the new pubkey.
3. Regenerate `config/services/maru/private-key`.
4. Regenerate the committed TLS material if the deployment is not purely local.
5. Start from an empty Docker volume so `account-setup` generates fresh runtime
   keys and derived addresses.

If you cannot tick these off, keep this stack on Sepolia.
