import assert from "node:assert/strict";
import test from "node:test";

import {
  canEnableSignButton,
  firstWalletAddress,
  nextWorkflowDisplayState,
  progressStageLabel,
  progressTone,
  scrollToHistoryFragment,
  signerAddressMismatch,
  signerUiSessionSecretStorageKey,
  signerTxFragmentId,
  signerUiHistoryStorageKey,
  signerUiLastSessionIdStorageKey,
  transactionExplorerUrl,
  transactionKindBadgeLabels,
  type WorkflowStatus,
  workflowStageLabel,
} from "../app/pageHelpers.ts";

test("progress helpers return expected labels and tones", () => {
  assert.equal(progressStageLabel("awaiting_wallet_approval"), "Awaiting wallet approval");
  assert.equal(progressStageLabel("submitted_waiting_for_rpc"), "Waiting for RPC broadcast");
  assert.equal(progressStageLabel("validating_submitted_transaction"), "Validating submitted transaction");
  assert.equal(progressStageLabel("waiting_for_hardhat_confirmation"), "Waiting for Hardhat confirmation");
  assert.equal(progressStageLabel("failed"), "Transaction failed");

  assert.equal(progressTone("awaiting_wallet_approval"), "active");
  assert.equal(progressTone("failed"), "error");
});

test("workflow status labels are stable", () => {
  assert.equal(workflowStageLabel("waiting_for_transaction_receipt"), "Waiting for transaction receipt");
  assert.equal(workflowStageLabel("waiting_for_contract_verification"), "Waiting for contract verification");
});

test("storage key helpers are deterministic", () => {
  assert.equal(
    signerUiHistoryStorageKey("http://127.0.0.1:15555", "session-123"),
    "signerUiTxHistory:http://127.0.0.1:15555:session-123",
  );
  assert.equal(
    signerUiLastSessionIdStorageKey("http://127.0.0.1:15555"),
    "signerUiLastSessionId:http://127.0.0.1:15555",
  );
  assert.equal(
    signerUiSessionSecretStorageKey("http://127.0.0.1:15555"),
    "signerUiSessionSecret:http://127.0.0.1:15555",
  );
});

test("transaction explorer url trims trailing slash and handles missing explorer", () => {
  assert.equal(transactionExplorerUrl(["https://etherscan.io/"], "0xabc"), "https://etherscan.io/tx/0xabc");
  assert.equal(transactionExplorerUrl([""], "0xabc"), null);
  assert.equal(transactionExplorerUrl([], "0xabc"), null);
});

test("fragment id helper sanitizes non-fragment-safe chars", () => {
  assert.equal(signerTxFragmentId("abc-123"), "signer-tx-abc-123");
  assert.equal(signerTxFragmentId("abc/123?x=1"), "signer-tx-abc-123-x-1");
});

test("wallet address helpers are resilient to missing or mismatched state", () => {
  assert.equal(firstWalletAddress(undefined), null);
  assert.equal(firstWalletAddress([]), null);
  assert.equal(firstWalletAddress(["0xabc", "0xdef"]), "0xabc");

  assert.equal(signerAddressMismatch(undefined, "0xabc"), false);
  assert.equal(signerAddressMismatch("0xabc", undefined), false);
  assert.equal(signerAddressMismatch("0xAbC", "0xaBc"), false);
  assert.equal(signerAddressMismatch("0xabc", "0xdef"), true);
});

test("sign button enablement follows connected provider address, not only wagmi session state", () => {
  assert.equal(
    canEnableSignButton({
      isSubmitting: false,
      walletUiReady: true,
      pendingRequestId: "request-1",
      connectedWalletAddress: "0xabc",
      expectedSignerAddress: null,
    }),
    true,
  );

  assert.equal(
    canEnableSignButton({
      isSubmitting: false,
      walletUiReady: true,
      pendingRequestId: "request-1",
      connectedWalletAddress: "0xabc",
      expectedSignerAddress: "0xdef",
    }),
    false,
  );

  assert.equal(
    canEnableSignButton({
      isSubmitting: false,
      walletUiReady: true,
      pendingRequestId: "request-1",
      connectedWalletAddress: null,
      expectedSignerAddress: null,
    }),
    false,
  );
});

test("transaction kind badges infer OpenZeppelin proxy and proxy admin", () => {
  assert.deepEqual(
    transactionKindBadgeLabels({
      openZeppelinProxyKind: "uups",
      contractName: "ProxyAdmin",
    }),
    ["UUPS proxy (OpenZeppelin)", "Proxy admin contract"],
  );
  assert.deepEqual(transactionKindBadgeLabels(null), []);
  assert.deepEqual(transactionKindBadgeLabels(undefined), []);
});

test("state status visible after source clears", () => {
  const receiptStatus: WorkflowStatus = {
    stage: "waiting_for_transaction_receipt",
    message: "Waiting for transaction receipt for TestContract.",
    updatedAt: "2026-04-01T10:00:00.000Z",
  };

  const shown = nextWorkflowDisplayState({
    incomingWorkflow: receiptStatus,
    currentWorkflow: null,
    currentExpiresAtMs: 0,
    nowMs: 1_000,
    minimumVisibilityMs: 2_000,
  });
  assert.deepEqual(shown, { workflow: receiptStatus, expiresAtMs: 3_000 });

  const stillVisible = nextWorkflowDisplayState({
    incomingWorkflow: null,
    currentWorkflow: shown.workflow,
    currentExpiresAtMs: shown.expiresAtMs,
    nowMs: 2_500,
    minimumVisibilityMs: 2_000,
  });
  assert.deepEqual(stillVisible, shown);

  const cleared = nextWorkflowDisplayState({
    incomingWorkflow: null,
    currentWorkflow: shown.workflow,
    currentExpiresAtMs: shown.expiresAtMs,
    nowMs: 3_001,
    minimumVisibilityMs: 2_000,
  });
  assert.deepEqual(cleared, { workflow: null, expiresAtMs: 0 });
});

test("workflow display state replaces stage immediately and resets minimum visibility timer", () => {
  const receiptStatus: WorkflowStatus = {
    stage: "waiting_for_transaction_receipt",
    message: "Waiting for transaction receipt.",
    updatedAt: "2026-04-01T10:00:00.000Z",
  };
  const verificationStatus: WorkflowStatus = {
    stage: "waiting_for_contract_verification",
    message: "Waiting for contract verification.",
    updatedAt: "2026-04-01T10:00:01.000Z",
  };

  const shown = nextWorkflowDisplayState({
    incomingWorkflow: receiptStatus,
    currentWorkflow: null,
    currentExpiresAtMs: 0,
    nowMs: 1_000,
    minimumVisibilityMs: 2_000,
  });

  const replaced = nextWorkflowDisplayState({
    incomingWorkflow: verificationStatus,
    currentWorkflow: shown.workflow,
    currentExpiresAtMs: shown.expiresAtMs,
    nowMs: 1_500,
    minimumVisibilityMs: 2_000,
  });

  assert.deepEqual(replaced, {
    workflow: verificationStatus,
    expiresAtMs: 3_500,
  });
});

test("scrollToHistoryFragment updates hash and attempts smooth scroll three times", () => {
  const originalWindow = globalThis.window;
  const originalDocument = globalThis.document;
  const originalRaf = globalThis.requestAnimationFrame;

  const replaceCalls: string[] = [];
  const scrollCalls: string[] = [];

  const mockWindow = {
    history: {
      replaceState: (_state: unknown, _title: string, url: string) => {
        replaceCalls.push(url);
      },
    },
    location: {
      pathname: "/",
      search: "?apiBaseUrl=http://127.0.0.1:15555",
    },
    setTimeout: (cb: () => void) => {
      cb();
      return 0 as unknown as number;
    },
  };

  const mockDocument = {
    getElementById: (id: string) => {
      if (id !== "signer-tx-abc") {
        return null;
      }
      return {
        scrollIntoView: () => {
          scrollCalls.push(id);
        },
      };
    },
  };

  Object.assign(globalThis, {
    window: mockWindow,
    document: mockDocument,
    requestAnimationFrame: (cb: FrameRequestCallback) => {
      cb(0);
      return 0;
    },
  });

  try {
    scrollToHistoryFragment("signer-tx-abc");
    assert.deepEqual(replaceCalls, ["/?apiBaseUrl=http://127.0.0.1:15555#signer-tx-abc"]);
    assert.equal(scrollCalls.length, 3);
  } finally {
    Object.assign(globalThis, {
      window: originalWindow,
      document: originalDocument,
      requestAnimationFrame: originalRaf,
    });
  }
});
