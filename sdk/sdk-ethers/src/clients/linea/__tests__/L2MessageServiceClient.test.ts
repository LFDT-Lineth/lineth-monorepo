import { describe, afterEach, it, expect, beforeEach } from "@jest/globals";
import { ContractTransactionResponse, Wallet } from "ethers";
import { MockProxy, mock, mockClear, mockDeep } from "jest-mock-extended";

import { L2MessageService, L2MessageService__factory } from "../../../contracts/typechain";
import { ZERO_ADDRESS } from "../../../core/constants";
import { OnChainMessageStatus } from "../../../core/enums/message";
import { BaseError, makeBaseError } from "../../../core/errors";
import {
  TEST_MESSAGE_HASH,
  TEST_CONTRACT_ADDRESS_1,
  TEST_TRANSACTION_HASH,
  TEST_ADDRESS_2,
  TEST_ADDRESS_1,
  DEFAULT_MAX_FEE_PER_GAS,
} from "../../../utils/testing/constants/common";
import {
  generateL2MessageServiceClient,
  generateMessage,
  generateTransactionResponse,
  mockProperty,
} from "../../../utils/testing/helpers";
import { GasProvider } from "../../gas";
import { LineaProvider } from "../../providers";
import { L2MessageServiceClient } from "../L2MessageServiceClient";

describe("TestL2MessageServiceClient", () => {
  let providerMock: MockProxy<LineaProvider>;
  let walletMock: MockProxy<Wallet>;
  let l2MessageServiceMock: MockProxy<L2MessageService>;

  let l2MessageServiceClient: L2MessageServiceClient;
  let gasFeeProvider: GasProvider;

  beforeEach(() => {
    providerMock = mock<LineaProvider>();
    walletMock = mock<Wallet>();
    l2MessageServiceMock = mockDeep<L2MessageService>();

    jest.spyOn(L2MessageService__factory, "connect").mockReturnValue(l2MessageServiceMock);
    walletMock.getAddress.mockResolvedValue(TEST_ADDRESS_1);
    l2MessageServiceMock.getAddress.mockResolvedValue(TEST_CONTRACT_ADDRESS_1);
    const clients = generateL2MessageServiceClient(providerMock, TEST_CONTRACT_ADDRESS_1, "read-write", walletMock);

    l2MessageServiceClient = clients.l2MessageServiceClient;
    gasFeeProvider = clients.gasProvider;
  });

  afterEach(() => {
    mockClear(providerMock);
    mockClear(walletMock);
    mockClear(l2MessageServiceMock);
    jest.clearAllMocks();
  });

  describe("constructor", () => {
    it("should throw an error when mode = 'read-write' and this.signer is undefined", async () => {
      expect(() => generateL2MessageServiceClient(providerMock, TEST_CONTRACT_ADDRESS_1, "read-write")).toThrowError(
        new BaseError("Please provide a signer."),
      );
    });
  });

  describe("getMessageStatus", () => {
    it("should return UNKNOWN when on chain message status === 0", async () => {
      jest.spyOn(l2MessageServiceMock, "inboxL1L2MessageStatus").mockResolvedValue(0n);

      const messageStatus = await l2MessageServiceClient.getMessageStatus({ messageHash: TEST_MESSAGE_HASH });

      expect(messageStatus).toStrictEqual(OnChainMessageStatus.UNKNOWN);
    });

    it("should return CLAIMABLE when on chain message status === 1", async () => {
      jest.spyOn(l2MessageServiceMock, "inboxL1L2MessageStatus").mockResolvedValue(1n);

      const messageStatus = await l2MessageServiceClient.getMessageStatus({ messageHash: TEST_MESSAGE_HASH });

      expect(messageStatus).toStrictEqual(OnChainMessageStatus.CLAIMABLE);
    });

    it("should return CLAIMED when on chain message status === 2", async () => {
      jest.spyOn(l2MessageServiceMock, "inboxL1L2MessageStatus").mockResolvedValue(2n);

      const messageStatus = await l2MessageServiceClient.getMessageStatus({ messageHash: TEST_MESSAGE_HASH });

      expect(messageStatus).toStrictEqual(OnChainMessageStatus.CLAIMED);
    });
  });

  describe("estimateClaimGasFees", () => {
    it("should throw an error when mode = 'read-only'", async () => {
      const l2MessageServiceClient = generateL2MessageServiceClient(
        providerMock,
        TEST_CONTRACT_ADDRESS_1,
        "read-only",
        walletMock,
      ).l2MessageServiceClient;
      const message = generateMessage();
      await expect(l2MessageServiceClient.estimateClaimGasFees(message)).rejects.toThrow(
        new Error("'EstimateClaimGasFees' function not callable using readOnly mode."),
      );
    });

    it("should throw a GasEstimationError when the gas estimation failed", async () => {
      const message = generateMessage();

      jest.spyOn(gasFeeProvider, "getGasFees").mockRejectedValue(new Error("Gas fees estimation failed").message);

      await expect(l2MessageServiceClient.estimateClaimGasFees(message)).rejects.toThrow(
        makeBaseError("Gas fees estimation failed", message),
      );
    });

    it("should set feeRecipient === ZeroAddress when feeRecipient param is undefined", async () => {
      const message = generateMessage();
      const transactionData = L2MessageService__factory.createInterface().encodeFunctionData("claimMessage", [
        message.messageSender,
        message.destination,
        message.fee,
        message.value,
        ZERO_ADDRESS,
        message.calldata,
        message.messageNonce,
      ]);
      mockProperty(l2MessageServiceMock, "interface", {
        encodeFunctionData: jest.fn().mockReturnValue(transactionData),
      } as any);

      const gasFeesSpy = jest.spyOn(gasFeeProvider, "getGasFees").mockResolvedValue({
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        gasLimit: 50_000n,
      });

      const estimatedGasFees = await l2MessageServiceClient.estimateClaimGasFees(message);

      expect(estimatedGasFees).toStrictEqual({
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        gasLimit: 50_000n,
      });

      expect(gasFeesSpy).toHaveBeenCalledTimes(1);
      expect(gasFeesSpy).toHaveBeenCalledWith({
        from: await walletMock.getAddress(),
        to: TEST_CONTRACT_ADDRESS_1,
        value: 0n,
        data: transactionData,
      });
    });

    it("should return estimated gas and fees for the claim message transaction", async () => {
      const message = generateMessage();
      const transactionData = L2MessageService__factory.createInterface().encodeFunctionData("claimMessage", [
        message.messageSender,
        message.destination,
        message.fee,
        message.value,
        TEST_ADDRESS_2,
        message.calldata,
        message.messageNonce,
      ]);
      mockProperty(l2MessageServiceMock, "interface", {
        encodeFunctionData: jest.fn().mockReturnValue(transactionData),
      } as any);

      const gasFeesSpy = jest.spyOn(gasFeeProvider, "getGasFees").mockResolvedValue({
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        gasLimit: 50_000n,
      });

      const estimatedGasFees = await l2MessageServiceClient.estimateClaimGasFees({
        ...message,
        feeRecipient: TEST_ADDRESS_2,
      });

      expect(estimatedGasFees).toStrictEqual({
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        gasLimit: 50_000n,
      });
      expect(gasFeesSpy).toHaveBeenCalledTimes(1);
      expect(gasFeesSpy).toHaveBeenCalledWith({
        from: await walletMock.getAddress(),
        to: TEST_CONTRACT_ADDRESS_1,
        value: 0n,
        data: transactionData,
      });
    });
  });

  describe("claim", () => {
    it("should throw an error when mode = 'read-only'", async () => {
      const l2MessageServiceClient = generateL2MessageServiceClient(
        providerMock,
        TEST_CONTRACT_ADDRESS_1,
        "read-only",
        walletMock,
      ).l2MessageServiceClient;
      const message = generateMessage();
      await expect(l2MessageServiceClient.claim(message)).rejects.toThrow(
        new Error("'claim' function not callable using readOnly mode."),
      );
    });

    it("should set feeRecipient === ZeroAddress when feeRecipient param is undefined", async () => {
      const message = generateMessage();
      const txResponse = generateTransactionResponse();
      jest.spyOn(l2MessageServiceMock, "claimMessage").mockResolvedValue(txResponse as ContractTransactionResponse);

      const claimMessageSpy = jest.spyOn(l2MessageServiceMock, "claimMessage");

      await l2MessageServiceClient.claim(message);

      expect(txResponse).toStrictEqual(txResponse);
      expect(claimMessageSpy).toHaveBeenCalledTimes(1);
      expect(claimMessageSpy).toHaveBeenCalledWith(
        message.messageSender,
        message.destination,
        message.fee,
        message.value,
        ZERO_ADDRESS,
        message.calldata,
        message.messageNonce,
        {},
      );
    });

    it("should return executed claim message transaction", async () => {
      const message = generateMessage();
      const txResponse = generateTransactionResponse();
      jest.spyOn(l2MessageServiceMock, "claimMessage").mockResolvedValue(txResponse as ContractTransactionResponse);

      const claimMessageSpy = jest.spyOn(l2MessageServiceMock, "claimMessage");

      await l2MessageServiceClient.claim({
        ...message,
        feeRecipient: TEST_ADDRESS_2,
      });

      expect(txResponse).toStrictEqual(txResponse);
      expect(claimMessageSpy).toHaveBeenCalledTimes(1);
      expect(claimMessageSpy).toHaveBeenCalledWith(
        message.messageSender,
        message.destination,
        message.fee,
        message.value,
        TEST_ADDRESS_2,
        message.calldata,
        message.messageNonce,
        {},
      );
    });
  });

  describe("retryTransactionWithHigherFee", () => {
    it("should throw an error when mode = 'read-only'", async () => {
      const l2MessageServiceClient = generateL2MessageServiceClient(
        providerMock,
        TEST_CONTRACT_ADDRESS_1,
        "read-only",
        walletMock,
      ).l2MessageServiceClient;

      await expect(l2MessageServiceClient.retryTransactionWithHigherFee(TEST_TRANSACTION_HASH)).rejects.toThrow(
        new BaseError("'retryTransactionWithHigherFee' function not callable using readOnly mode."),
      );
    });

    it("should throw an error when priceBumpPercent is not an integer", async () => {
      await expect(l2MessageServiceClient.retryTransactionWithHigherFee(TEST_TRANSACTION_HASH, 1.1)).rejects.toThrow(
        new BaseError("'priceBumpPercent' must be an integer"),
      );
    });

    it("should throw an error when getTransaction return null", async () => {
      jest.spyOn(providerMock, "getTransaction").mockResolvedValue(null);

      await expect(l2MessageServiceClient.retryTransactionWithHigherFee(TEST_TRANSACTION_HASH)).rejects.toThrow(
        new BaseError(`Transaction with hash ${TEST_TRANSACTION_HASH} not found.`),
      );
    });

    it("should retry the transaction with higher fees", async () => {
      const transactionResponse = generateTransactionResponse();
      const getTransactionSpy = jest.spyOn(providerMock, "getTransaction").mockResolvedValue(transactionResponse);
      const signTransactionSpy = jest.spyOn(walletMock, "signTransaction").mockResolvedValue("");
      const sendTransactionSpy = jest.spyOn(providerMock, "broadcastTransaction");

      await l2MessageServiceClient.retryTransactionWithHigherFee(TEST_TRANSACTION_HASH);

      expect(getTransactionSpy).toHaveBeenCalledTimes(1);
      expect(signTransactionSpy).toHaveBeenCalledTimes(1);
      expect(signTransactionSpy).toHaveBeenCalledWith({
        to: transactionResponse.to,
        value: transactionResponse.value,
        data: transactionResponse.data,
        nonce: transactionResponse.nonce,
        gasLimit: transactionResponse.gasLimit,
        chainId: transactionResponse.chainId,
        type: 2,
        maxPriorityFeePerGas: 55000000n,
        maxFeePerGas: 110000000n,
      });
      expect(sendTransactionSpy).toHaveBeenCalledTimes(1);
    });

    it("should retry the transaction with higher fees and capped by the predefined maxFeePerGas", async () => {
      const transactionResponse = generateTransactionResponse();
      const getTransactionSpy = jest.spyOn(providerMock, "getTransaction").mockResolvedValue(transactionResponse);
      const signTransactionSpy = jest.spyOn(walletMock, "signTransaction").mockResolvedValue("");
      const sendTransactionSpy = jest.spyOn(providerMock, "broadcastTransaction");

      const l2MessageServiceClient = generateL2MessageServiceClient(
        providerMock,
        TEST_CONTRACT_ADDRESS_1,
        "read-write",
        walletMock,
        { maxFeePerGasCap: 500000000n },
      ).l2MessageServiceClient;

      await l2MessageServiceClient.retryTransactionWithHigherFee(TEST_TRANSACTION_HASH, 1000);

      expect(getTransactionSpy).toHaveBeenCalledTimes(1);
      expect(signTransactionSpy).toHaveBeenCalledTimes(1);
      expect(signTransactionSpy).toHaveBeenCalledWith({
        to: transactionResponse.to,
        value: transactionResponse.value,
        data: transactionResponse.data,
        nonce: transactionResponse.nonce,
        gasLimit: transactionResponse.gasLimit,
        chainId: transactionResponse.chainId,
        type: 2,
        maxPriorityFeePerGas: 500000000n,
        maxFeePerGas: 500000000n,
      });
      expect(sendTransactionSpy).toHaveBeenCalledTimes(1);
    });

    it("should retry the transaction with the predefined maxFeePerGas if enforceMaxGasFee is true", async () => {
      const transactionResponse = generateTransactionResponse({
        maxPriorityFeePerGas: undefined,
        maxFeePerGas: undefined,
      });
      const getTransactionSpy = jest.spyOn(providerMock, "getTransaction").mockResolvedValue(transactionResponse);
      const signTransactionSpy = jest.spyOn(walletMock, "signTransaction").mockResolvedValue("");
      const sendTransactionSpy = jest.spyOn(providerMock, "broadcastTransaction");
      const getFeeDataSpy = jest.spyOn(providerMock, "getFees").mockResolvedValueOnce({
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
      });

      const clients = generateL2MessageServiceClient(providerMock, TEST_CONTRACT_ADDRESS_1, "read-write", walletMock, {
        maxFeePerGasCap: 500000000n,
        enforceMaxGasFee: true,
      });

      const l2MessageServiceClient = clients.l2MessageServiceClient;

      const providerSendMockSpy = jest.spyOn(providerMock, "send").mockResolvedValue({
        baseFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        priorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        gasLimit: 50_000n,
      });

      await l2MessageServiceClient.retryTransactionWithHigherFee(TEST_TRANSACTION_HASH, 1000);
      expect(providerSendMockSpy).toHaveBeenCalledTimes(0);
      expect(getTransactionSpy).toHaveBeenCalledTimes(1);
      expect(signTransactionSpy).toHaveBeenCalledTimes(1);
      expect(getFeeDataSpy).toHaveBeenCalledTimes(1);
      expect(signTransactionSpy).toHaveBeenCalledWith({
        to: transactionResponse.to,
        value: transactionResponse.value,
        data: transactionResponse.data,
        nonce: transactionResponse.nonce,
        gasLimit: transactionResponse.gasLimit,
        chainId: transactionResponse.chainId,
        type: 2,
        maxPriorityFeePerGas: 100000000000n,
        maxFeePerGas: 100000000000n,
      });
      expect(sendTransactionSpy).toHaveBeenCalledTimes(1);
    });
  });

  describe("isRateLimitExceeded", () => {
    it("should always return false", async () => {
      const isRateLimitExceeded = await l2MessageServiceClient.isRateLimitExceeded(1000000000n, 1000000000n);
      expect(isRateLimitExceeded).toBeFalsy();
    });
  });

  describe("isRateLimitExceededError", () => {
    it("should return false when something went wrong (http error etc)", async () => {
      jest.spyOn(providerMock, "getTransaction").mockRejectedValueOnce({});
      expect(
        await l2MessageServiceClient.isRateLimitExceededError(
          "0x825a7f1aa4453735597ddf7e9062413c906a7ad49bf17ff32c2cf42f41d438d9",
        ),
      ).toStrictEqual(false);
    });

    it("should return false when transaction revert reason is not RateLimitExceeded", async () => {
      jest.spyOn(providerMock, "getTransaction").mockResolvedValueOnce(generateTransactionResponse());
      jest.spyOn(providerMock, "call").mockResolvedValueOnce("0xa74c1c6d");

      expect(
        await l2MessageServiceClient.isRateLimitExceededError(
          "0x825a7f1aa4453735597ddf7e9062413c906a7ad49bf17ff32c2cf42f41d438d9",
        ),
      ).toStrictEqual(false);
    });

    it("should return true when transaction revert reason is RateLimitExceeded", async () => {
      mockProperty(l2MessageServiceMock, "interface", {
        ...l2MessageServiceMock.interface,
        parseError: jest.fn().mockReturnValueOnce({ name: "RateLimitExceeded" }),
      } as any);
      jest.spyOn(providerMock, "getTransaction").mockResolvedValueOnce(generateTransactionResponse());
      jest.spyOn(providerMock, "call").mockResolvedValueOnce("0xa74c1c5f");

      expect(
        await l2MessageServiceClient.isRateLimitExceededError(
          "0x825a7f1aa4453735597ddf7e9062413c906a7ad49bf17ff32c2cf42f41d438d9",
        ),
      ).toStrictEqual(true);
    });
  });
});
