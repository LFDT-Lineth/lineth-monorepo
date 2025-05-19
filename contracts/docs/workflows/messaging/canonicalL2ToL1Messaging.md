
# 📩 Interaction Flow: Canonical Message Sending (L2 → L1)

This document describes the step-by-step flow of how a canonical message is sent from L2 to L1 in the Linea network.

---

## 🔄 Step-by-Step Flow

1. **L2 User** calls `sendMessage()` on the `L2MessageService`.
2. The contract:
   - Verifies non-empty data
   - Gets the next message number
   - Computes the message hash with all the message fields
   - Stores the message hash and emits events
3. **Coordinator** captures the event and message hash.
4. Coordinator:
   - Anchors the messaging Merkle root(s) on L1 during finalization
   - Emits events for the L2 user to construct a Merkle proof
5. **L1 User** claims the message with proof on the `LineaRollup` / `L1MessageService`.
6. The contract verifies and **delivers the message** to the recipient.

---

## 🖼️ Diagram

<img src="../diagrams/canonicalL2ToL1Messaging.png">