
# 🔄 Upgrading a Contract Without Calling a Reinitialize Function

This guide outlines the secure process for upgrading a contract (e.g. **LineaRollup**, **L2MessageService**, **TokenBridge**) without calling a reinitialize function, via a timelock-protected upgrade flow.

**Note**: These contracts are governed by the [Security Council Charter](../../security-council-charter.md).

---

## 🟨 1. Deploy Implementation of Upgradable Contract

**Actor:** Well-Known Deployer  
**Actions:**

- Deploys a new `Implementation` contract (e.g. `0xAbCd1234`)
- The implementation:
  - ✅ Must be **verified on Etherscan or Linescan**
  - ✅ Must match **audited code** confirmed by multiple auditors
  - ✅ Bytecode must be **verified** to match the audit

## 🗂️ Function Signatures

| 4bytes | Signature                              |
|-------|---------------------------------------|
| `0x99a88ec4`     | upgrade(address,address)                   |

---

## 🟧 2. Schedule Upgrade Transaction

**Timeframe:** Some time after deployment  
**Actor:** Council Member  
**Actions:**

- Adds a **scheduled upgrade transaction** via the `Security Council Safe`
- The transaction:
  - Is routed through the `TimelockController`
  - Targets the `ProxyAdmin`
  - Calls the `upgrade()` function to set the proxy’s implementation to the new contract address (e.g. `0xAbCd1234`)
  - Final target: the `Proxy` contract

**Verification Requirements:**
- ✅ Transaction hash, details, and simulation must be verified
- ✅ Function and parameters must be verified

---

## 🟩 3. Execute Upgrade Transaction After Delay

**Timeframe:** After the configured delay  
**Actor:** Council Member  
**Actions:**

- Adds a matching **execute transaction** through the `Security Council Safe`
- Routed again through the `TimelockController`
- Reaches `ProxyAdmin`, which executes the upgrade on the `Proxy`

**Outcome:**  
➡️ Proxy’s implementation is updated (contract logic changed)

**Verification Requirements:**
- ✅ Same transaction hash, parameters, and targets must match scheduled version

---

## 🗂️ Mainnet Contract Addresses

### 🔐 Security Council Addresses

| Network   | Address                                      |
|-----------|----------------------------------------------|
| Ethereum  | `0x892bb7EeD71efB060ab90140e7825d8127991DD3` |
| Linea     | `0xf5cc7604a5ef3565b4D2050D65729A06B68AA0bD` |

### 🕓 TimelockController Addresses

| Network   | Address                                      |
|-----------|----------------------------------------------|
| Ethereum  | `0xd6B9c960f779c728C6752119849318E5d550574`  |
| Linea     | `0xc808BfCBeD34D90fa9579CAa664e67B9A03C56ca` |

- Security Council is both **Proposer** and **Executor**
- Timelock owns `ProxyAdmin` contracts

### 👤 Proxy Admin Addresses

| Network   | Address                                      |
|-----------|----------------------------------------------|
| Ethereum  | `0xF5058616517C068C7b8c7EbC69FF636Ade9066d6` |
| Linea     | `0x1E1f6F22f97b4a7522D8B62e983953639239774E` |

### 🧑‍💻 Deployer Addresses

| Network   | Address                                      |
|-----------|----------------------------------------------|
| Ethereum  | `0x6dD3120E329dC5FaA3d2Cf65705Ef4f6486F65F7` |
| Linea     | `0x49ee40140E522651744e1C27828c76eE92802833` |

### 📦 Proxy Addresses

| Contract           | Address                                           |
|--------------------|---------------------------------------------------|
| LineaRollup        | `0xd19d4B5d358258f05D7B411E21A1460D11B0876F`       |
| TokenBridge        | `0x051F1D88f0aF5763fB888eC4378b4D8B29ea3319`       |
| L2MessageService   | `0x508Ca82Df566dCD1B0DE8296e70a96332cD644ec`      |
| L2 Token Bridge    | `0x353012dc4a9A6cF55c941bADC267f82004A8ceB9`        |

---

## ✅ Security Summary

- All upgrades go through a **timelock delay** using multisig control
- Contract upgrades do **not require reinitialization**
- **Transaction simulation and parameter verification** are essential at both stages

<img src="../diagrams/upgradeContract.png">