import { IContractSignerClient } from "@consensys/linea-shared-utils";
import { describe, it, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";
import { parseTransaction, type PublicClient, serializeSignature, type WalletClient } from "viem";
import { privateKeyToAccount } from "viem/accounts";

import { ILineaGasProvider } from "../../../core/clients/blockchain/IGasProvider";
import {
  DEFAULT_MAX_FEE_PER_GAS,
  TEST_CONTRACT_ADDRESS_1,
  TEST_L2_SIGNER_PRIVATE_KEY,
  testMessage,
} from "../../../utils/testing/constants";
import { L2ClaimTransactionSizeCalculator } from "../L2ClaimTransactionSizeCalculator";
import { ViemL2MessageServiceClient } from "../viem/clients/ViemL2MessageServiceClient";
import { ViemTransactionSigner } from "../viem/signers/ViemTransactionSigner";

const account = privateKeyToAccount(TEST_L2_SIGNER_PRIVATE_KEY);

describe("L2ClaimTransactionSizeCalculator", () => {
  let transactionSizeCalculator: L2ClaimTransactionSizeCalculator;

  const signerClient = mock<IContractSignerClient>();

  const l2ContractClient = new ViemL2MessageServiceClient(
    mock<PublicClient>(),
    mock<WalletClient>(),
    TEST_CONTRACT_ADDRESS_1,
    mock<ILineaGasProvider>(),
    account.address,
  );

  beforeEach(() => {
    // Sign locally without any RPC call, returning only the signature hex
    signerClient.sign.mockImplementation(async (tx) => {
      const serializedSignedTx = await account.signTransaction(tx);
      const { r, s, yParity } = parseTransaction(serializedSignedTx);
      return serializeSignature({ r: r!, s: s!, yParity: yParity! });
    });

    const transactionSigner = new ViemTransactionSigner(signerClient, 1337);
    transactionSizeCalculator = new L2ClaimTransactionSizeCalculator(l2ContractClient, transactionSigner);
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  describe("process", () => {
    it("Should return transaction size", async () => {
      const transactionSize = await transactionSizeCalculator.calculateTransactionSize(testMessage, {
        gasLimit: 50_000n,
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
      });

      expect(transactionSize).toEqual(83);
    });
  });
});
