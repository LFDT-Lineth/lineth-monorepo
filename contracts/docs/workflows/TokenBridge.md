# 📘 Canonical Token Bridge Core Admin Workflows

This collection of guides outlines key privileged workflows that govern the operation and upgrade lifecycle of the Canonical Token Bridge smart contract. These workflows are designed to be executed via multisig-controlled safes, with strong verification and simulation practices to ensure safety and auditability.

Each section below links to a dedicated Markdown document detailing that category of administrative action.

---

## 📑 Workflows

### 1. 🔐 [Role Management](./administration/roleManagement.md)
Granting or revoking operational roles on core contracts like LinethRollup, L2MessageService, and TokenBridge.

### 2. ⏸️ [Pausing Features](./administration/pausing.md)
How to pause contract functionality using well-defined pause types.

### 3. ▶️ [Unpausing Features](./administration/unpausing.md)
How to resume paused features using the same set of pause types.

### 4. 🧮 [Rate Limiting](./administration/rateLimiting.md)
How to configure or reset message throughput limits.

### 5. ♻️ [Upgrading Without Reinitialization](./administration/upgradeContract.md)
Securely upgrade the Canonical Token Bridge or related contracts without calling reinitialization logic.

### 6. 🔁 [Upgrading With Reinitialization](./administration/upgradeAndCallContract.md)
Same upgrade flow but includes immediate initialization logic for the new implementation.

### 7. 🧾 [Other Token Bridge Specific Admin Functions](./administration/tokenBridge.md)
How to execute `setCustomContract`,`removeReserved`, `setMessageService` and `setReserved`

### 8. 🧾 [L1 to L2 Token Bridging](./messaging/canonicalL1ToL2TokenBridging.md)
View the L1 to L2 Token Bridging flow.

### 9. 🧾 [L2 to L1 Token Bridging](./messaging/canonicalL2ToL1TokenBridging.md)
View the L2 to L1 Token Bridging flow.
---

## ✅ Notes

- All workflows require transaction simulation and parameter verification.
- Some admin operations are governed by time-locked multisigs.
- Contract addresses and roles are listed inline in each document.

