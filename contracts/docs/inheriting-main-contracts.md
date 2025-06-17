# Customizing Linea Rollup, Messaging, and Bridging Components

This guide explains how to customize core rollup, messaging, and bridging functionality by overriding virtual functions in the Linea protocol contracts. Several examples are provided to help you get started. Please note, these are just illustrative samples.

**Note:** It is recommended that any overrides or modifications from the original audited code should be independently audited for conflicting behavior.

## 🧩 Customizing Contract Versions

To modify the ABI version returned, the `CONTRACT_VERSION()` must be overriden with your specific string value. The default is the base version.

## 🧱 Customizing Linea Rollup Behavior

To modify the behavior of the rollup mechanism itself (e.g. blob submission for finalization), you can override virtual functions in `LineaRollup.sol`.

## ✉️ Customizing Message Sending Behavior

The `MessageService` contract handles L1 ↔ L2 message passing. If you want to adjust how messages are sent or verified (e.g. adding rules, changing fee logic), you can override its virtual functions.

See [`InheritingLineaRollup.sol`](../src/_testing/unit/rollup/InheritingLineaRollup.sol) for an example where `MessageService` functionality is customized within the rollup context.

The example provided prevents fee and value from being directly sent to the message service, but instead relies on the samples for the generic Ether bridge - sample are found links below.

## 🔁 Overriding Token Bridge Behavior

To implement custom behavior for token bridging (e.g., additional validation, whitelisting, alternative transfer mechanisms), override functions in the `TokenBridge` contract.

A reference implementation is available in [`InheritingTokenBridge.sol`](../src/_testing/unit/bridging/InheritingTokenBridge.sol), demonstrating how to override core bridge functions for ERC20 tokens.

The example creates a generic way move specific funds to alternate escrow accounts when bridging. This could be done for various reasons - e.g. independently controlled asset types.

## 📨 Overriding L2 Message Service Behavior

If you need to modify how messages are processed or validated on the L2 side, override functions in the `L2MessageService` contract.

Check out [`InheritingL2MessageService.sol`](../src/_testing/unit/messaging/InheritingL2MessageService.sol) for a practical example of extending L2 message processing behavior.

This is modified in line with the customized L1MessageService behavior to work with the custom Ether bridges.

## 🪙 Custom Ether Bridge Examples

For more advanced use cases involving Ether transfers—such as delayed withdrawals or multi-phase bridging—you can extend and override the send/receive logic in:

- [`L1GenericBridge.sol`](../src/_testing/unit/bridging/l1/L1GenericBridge.sol)
- [`L2GenericBridge.sol`](../src/_testing/unit/bridging/l2/L2GenericBridge.sol)

These sample files serve as a foundation for creating fully custom Ether bridging flows tailored to your application needs.

---

By leveraging inheritance and Solidity's `virtual`/`override` keywords, you can safely extend and adapt Linea's modular bridge and rollup system.

Additionally, it is worth noting that all the inherited contracts now contain an additional 50 storage slots of padded space for future Linea expansion without breaking the underlying inheritors layouts.
