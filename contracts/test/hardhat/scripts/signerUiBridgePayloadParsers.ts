import { expect } from "chai";

import { __testOnlySignerUiBridge } from "../../../scripts/hardhat/signer-ui-bridge";

describe("signer-ui bridge payload parsing and request shaping", () => {
  it("parses wallet payload with valid address and integer chain id", () => {
    const parsed = __testOnlySignerUiBridge.parseWalletPayload({
      address: "0x000000000000000000000000000000000000dEaD",
      chainId: 59141,
    });

    expect(parsed.ok).to.equal(true);
    if (!parsed.ok) {
      return;
    }

    expect(parsed.value.address).to.equal("0x000000000000000000000000000000000000dEaD");
    expect(parsed.value.chainId).to.equal(59141);
  });

  it("rejects wallet payload with invalid chain id type", () => {
    const parsed = __testOnlySignerUiBridge.parseWalletPayload({
      address: "0x000000000000000000000000000000000000dEaD",
      chainId: "59141",
    });

    expect(parsed.ok).to.equal(false);
    expect(parsed).to.deep.equal({ ok: false, error: "Invalid chainId." });
  });

  it("parses respond payload and rejects malformed payloads", () => {
    const valid = __testOnlySignerUiBridge.parseRespondPayload({
      requestId: "abc",
      hash: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      from: "0x000000000000000000000000000000000000dEaD",
      chainId: 59141,
    });
    expect(valid.ok).to.equal(true);

    const badHash = __testOnlySignerUiBridge.parseRespondPayload({
      requestId: "abc",
      hash: "0x1234",
      from: "0x000000000000000000000000000000000000dEaD",
      chainId: 59141,
    });
    expect(badHash).to.deep.equal({ ok: false, error: "Missing or invalid transaction hash." });

    const badRequest = __testOnlySignerUiBridge.parseRespondPayload(null);
    expect(badRequest).to.deep.equal({ ok: false, error: "Invalid JSON body." });
  });

  it("parses error payload and truncates message to 4000 chars", () => {
    const longMessage = "x".repeat(4500);
    const parsed = __testOnlySignerUiBridge.parseErrorPayload({
      requestId: "abc",
      message: longMessage,
    });

    expect(parsed.ok).to.equal(true);
    if (!parsed.ok) {
      return;
    }

    expect(parsed.value.message.length).to.equal(4000);
    expect(parsed.value.requestId).to.equal("abc");
  });

  it("normalizes fee fields based on tx type", () => {
    expect(
      __testOnlySignerUiBridge.normalizeGasFeeFieldsForWallet({
        gasPrice: "0x1",
        maxFeePerGas: "0x2",
        maxPriorityFeePerGas: "0x3",
        type: 0,
      }),
    ).to.deep.equal({ gasPrice: "0x1" });

    expect(
      __testOnlySignerUiBridge.normalizeGasFeeFieldsForWallet({
        gasPrice: "0x1",
        maxFeePerGas: "0x2",
        maxPriorityFeePerGas: "0x3",
        type: 2,
      }),
    ).to.deep.equal({
      maxFeePerGas: "0x2",
      maxPriorityFeePerGas: "0x3",
    });

    expect(
      __testOnlySignerUiBridge.normalizeGasFeeFieldsForWallet({
        gasPrice: "0x1",
        maxFeePerGas: undefined,
        maxPriorityFeePerGas: undefined,
        type: undefined,
      }),
    ).to.deep.equal({ gasPrice: "0x1" });
  });

  it("omits undefined optional tx request fields", () => {
    expect(
      __testOnlySignerUiBridge.buildSerializedTransactionRequest({
        to: "0x000000000000000000000000000000000000dEaD",
        data: undefined,
        gasPrice: "0x1",
      }),
    ).to.deep.equal({
      to: "0x000000000000000000000000000000000000dEaD",
      gasPrice: "0x1",
    });
  });

  it("enforces minimum dwell time when workflow stage transitions", () => {
    expect(
      __testOnlySignerUiBridge.workflowStatusTransitionDelayMs({
        currentStage: "waiting_for_transaction_receipt",
        nextStage: "waiting_for_contract_verification",
        currentStageChangedAtMs: 1_000,
        nowMs: 1_500,
        minimumStageMs: 2_000,
      }),
    ).to.equal(1_500);

    expect(
      __testOnlySignerUiBridge.workflowStatusTransitionDelayMs({
        currentStage: "waiting_for_transaction_receipt",
        nextStage: "waiting_for_contract_verification",
        currentStageChangedAtMs: 1_000,
        nowMs: 3_100,
        minimumStageMs: 2_000,
      }),
    ).to.equal(0);
  });

  it("does not delay when staying on same stage or when no current stage exists", () => {
    expect(
      __testOnlySignerUiBridge.workflowStatusTransitionDelayMs({
        currentStage: "waiting_for_transaction_receipt",
        nextStage: "waiting_for_transaction_receipt",
        currentStageChangedAtMs: 1_000,
        nowMs: 1_500,
        minimumStageMs: 2_000,
      }),
    ).to.equal(0);

    expect(
      __testOnlySignerUiBridge.workflowStatusTransitionDelayMs({
        currentStage: null,
        nextStage: "waiting_for_transaction_receipt",
        currentStageChangedAtMs: 0,
        nowMs: 1_000,
        minimumStageMs: 2_000,
      }),
    ).to.equal(0);
  });
});
