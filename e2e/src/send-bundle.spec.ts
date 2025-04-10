import { describe, expect, it } from "@jest/globals";
import { config } from "./config/tests-config";
import {
  generateRandomUUIDv4,
  getRawTransactionHex,
  getTransactionHash,
  getWallet,
  LineaBundleClient,
  pollForBlockNumber,
} from "./common/utils";
import { ethers, TransactionRequest } from "ethers";

const describeIf = shouldSkipBundleTests ? describe.skip : describe;
if (shouldSkipBundleTests) {
  logger.info("Skip bundle tests due to tracing-v1 besu nodes");
}

describeIf("Send bundle test suite", () => {
  const l2AccountManager = config.getL2AccountManager();
  const lineaCancelBundleClient = new LineaBundleClient(config.getSequencerEndpoint()!);
  const lineaSendBundleClient = new LineaBundleClient(config.getL2BesuNodeEndpoint()!);

  it.concurrent(
    "Call sendBundle to RPC node and the bundled txs should get included",
    async () => {
      const senderAccount = await l2AccountManager.generateAccount();
      const senderWallet = getWallet(senderAccount.privateKey, config.getL2BesuNodeProvider()!);
      const recipientAccount = await l2AccountManager.generateAccount(0n);

      let senderNonce = await senderAccount.getNonce();
      const txHashes: string[] = [];
      const txs: string[] = [];
      for (let i = 0; i < 3; i++) {
        const txRequest: TransactionRequest = {
          to: recipientAccount.address,
          value: ethers.parseUnits("1000", "wei"),
          maxPriorityFeePerGas: ethers.parseEther("0.000000001"), // 1 Gwei
          maxFeePerGas: ethers.parseEther("0.00000001"), // 10 Gwei
          nonce: senderNonce++,
        };
        txs.push(await getRawTransactionHex(txRequest, senderWallet));
        txHashes.push(await getTransactionHash(txRequest, senderWallet));
      }
      const targetBlockNumber = (await config.getL2Provider().getBlockNumber()) + 5;
      const replacementUUID = generateRandomUUIDv4();

      await lineaSendBundleClient.lineaSendBundle(txs, replacementUUID, "0x" + targetBlockNumber.toString(16));

      const hasReachedTargeBlockNumber = await pollForBlockNumber(config.getL2Provider(), targetBlockNumber);

      expect(hasReachedTargeBlockNumber).toBeTruthy();
      for (const tx of txHashes) {
        const receipt = await config.getL2Provider().getTransactionReceipt(tx);
        expect(receipt?.status).toStrictEqual(1);
      }
    },
    120_000,
  );

  it.concurrent(
    "Call sendBundle to RPC node but the bundled txs should not get included as not all of them is valid",
    async () => {
      // 1500 wei should just be enough for the first ETH transfer tx, and the second and third would fail
      const senderAccount = await l2AccountManager.generateAccount(ethers.parseUnits("1500", "wei"));
      const senderWallet = getWallet(senderAccount.privateKey, config.getL2BesuNodeProvider()!);
      const recipientAccount = await l2AccountManager.generateAccount(0n);

      let senderNonce = await senderAccount.getNonce();
      const txHashes: string[] = [];
      const txs: string[] = [];
      for (let i = 0; i < 3; i++) {
        const txRequest: TransactionRequest = {
          to: recipientAccount.address,
          value: ethers.parseUnits("1000", "wei"),
          maxPriorityFeePerGas: ethers.parseEther("0.000000001"), // 1 Gwei
          maxFeePerGas: ethers.parseEther("0.00000001"), // 10 Gwei
          nonce: senderNonce++,
        };
        txs.push(await getRawTransactionHex(txRequest, senderWallet));
        txHashes.push(await getTransactionHash(txRequest, senderWallet));
      }
      const targetBlockNumber = (await config.getL2Provider().getBlockNumber()) + 5;
      const replacementUUID = generateRandomUUIDv4();

      await lineaSendBundleClient.lineaSendBundle(txs, replacementUUID, "0x" + targetBlockNumber.toString(16));

      const hasReachedTargeBlockNumber = await pollForBlockNumber(config.getL2Provider(), targetBlockNumber);

      expect(hasReachedTargeBlockNumber).toBeTruthy();
      // None of the bundled txs should be included as not all of them is valid
      for (const tx of txHashes) {
        const receipt = await config.getL2Provider().getTransactionReceipt(tx);
        expect(receipt?.status).toBeUndefined();
      }
    },
    120_000,
  );

  it.concurrent(
    "Call sendBundle to RPC node and then cancelBundle to sequencer and no bundled txs should get included",
    async () => {
      const senderAccount = await l2AccountManager.generateAccount();
      const senderWallet = getWallet(senderAccount.privateKey, config.getL2BesuNodeProvider()!);
      const recipientAccount = await l2AccountManager.generateAccount(0n);

      let senderNonce = await senderAccount.getNonce();
      const txHashes: string[] = [];
      const txs: string[] = [];
      for (let i = 0; i < 3; i++) {
        const txRequest: TransactionRequest = {
          to: recipientAccount.address,
          value: ethers.parseUnits("1000", "wei"),
          maxPriorityFeePerGas: ethers.parseEther("0.000000001"), // 1 Gwei
          maxFeePerGas: ethers.parseEther("0.00000001"), // 10 Gwei
          nonce: senderNonce++,
        };
        txs.push(await getRawTransactionHex(txRequest, senderWallet));
        txHashes.push(await getTransactionHash(txRequest, senderWallet));
      }
      const targetBlockNumber = (await config.getL2Provider().getBlockNumber()) + 10;
      const replacementUUID = generateRandomUUIDv4();

      await lineaSendBundleClient.lineaSendBundle(txs, replacementUUID, "0x" + targetBlockNumber.toString(16));

      await pollForBlockNumber(config.getL2Provider(), targetBlockNumber - 5);
      const cancelled = await lineaCancelBundleClient.lineaCancelBundle(replacementUUID);
      expect(cancelled).toBeTruthy();

      const hasReachedTargeBlockNumber = await pollForBlockNumber(config.getL2Provider(), targetBlockNumber);

      expect(hasReachedTargeBlockNumber).toBeTruthy();
      for (const tx of txHashes) {
        const receipt = await config.getL2Provider().getTransactionReceipt(tx);
        expect(receipt?.status).toBeUndefined();
      }
    },
    120_000,
  );
});
