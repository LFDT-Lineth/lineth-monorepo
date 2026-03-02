
# 🛠️ Executing TokenBridge Admin Functions (Excluding Pausing)

This guide outlines how authorized safe members can execute core **TokenBridge admin functions** like setting or removing reserved addresses or setting custom contracts and message services.

**Note**: These contracts are governed by the [Security Council Charter](../../security-council-charter.md).

---

## 🟧 Flow: Admin Execution Path

**Actor:** Safe Member  
**Actions:**

- Adds a transaction via **Security Council** or **Operational Safe**
- Targets the relevant **TokenBridge Proxy**
- Calls one of the following functions:
  - `setCustomContract`
  - `removeReserved`
  - `setMessageService`
  - `setReserved`

**Execution Path:**
```
Safe Member
    → Security Council / Operational Safe
        → targets Proxy
            → calls admin function
                → signs and executes on-chain
```

**Verification Requirements:**
- ✅ Transaction hash, parameters, and simulation must be verified

## 🗂️ Function Signatures

| 4bytes | Signature                              |
|-------|---------------------------------------|
| `0x1754f301`    | setCustomContract(address,address)                   |
| `0xedc42a22`    | removeReserved(address)                   |
| `0xbe46096f`    | setMessageService(address)                   |
| `0xcdd914c5`    | setReserved(address)                   |

---

## 🗂️ All Mainnet Contract Addresses

### 🔐 Security Council Addresses

| Network   | Address                                      |
|-----------|----------------------------------------------|
| Ethereum  | `0x892bb7EeD71efB060ab90140e7825d8127991DD3` |
| Linea     | `0xf5cc7604a5ef3565b4D2050D65729A06B68AA0bD` |

### 📦 Proxy Addresses

| Contract         | Address                                           |
|------------------|---------------------------------------------------|
| L1 TokenBridge   | `0x051F1D88f0aF5763fB888eC4378b4D8B29ea3319`       |
| L2 Token Bridge  | `0x353012dc4a9A6cF55c941bADC267f82004A8ceB9`        |

---

## ✅ Security Summary

- Admin functions require **authorized multisig approval**
- All actions are **on-chain and irreversible**
- **Parameter verification and simulation** are mandatory before execution


<img src="../diagrams/tokenBridge.png">
