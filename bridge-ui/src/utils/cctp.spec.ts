import { test } from "@playwright/test";
import nock from "nock";
import { getCctpMessageExpiryBlock, getCctpTransactionStatus } from "./cctp";
import {
  CctpAttestationMessage,
  CctpAttestationMessageStatus,
  CctpV2ReattestationApiResponse,
  Chain,
  ChainLayer,
  TransactionStatus,
} from "@/types";

const { expect, describe } = test;

describe("getCctpMessageExpiryBlock", () => {
  test("should return undefined for empty byte string", () => {
    const message = "0x";
    const resp = getCctpMessageExpiryBlock(message);
    expect(resp).toBeUndefined();
  });

  test("should return 0 if 0 expiryBlock encoded", () => {
    const message =
      "0x00000001000000000000000b41d77498ae6f504499ff1ead8c1c2a3318d48063b8022f8215f7631153534d210000000000000000000000008fe6b999dc680ccfdd5bf7eb0974218be2542daa0000000000000000000000008fe6b999dc680ccfdd5bf7eb0974218be2542daa0000000000000000000000000000000000000000000000000000000000000000000003e8000007d0000000010000000000000000000000001c7d4b196cb0c7b01d743fbc6116a902379c7238000000000000000000000000558d9534cac58f743a3a9e5382f77575a2595dcb0000000000000000000000000000000000000000000000000000000000002710000000000000000000000000558d9534cac58f743a3a9e5382f77575a2595dcb000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000000";
    const resp = getCctpMessageExpiryBlock(message);
    expect(resp).toBe(BigInt(0));
  });

  test("should parse encoded expiryBlock", () => {
    const message =
      "0x00000001000000000000000b41d77498ae6f504499ff1ead8c1c2a3318d48063b8022f8215f7631153534d210000000000000000000000008fe6b999dc680ccfdd5bf7eb0974218be2542daa0000000000000000000000008fe6b999dc680ccfdd5bf7eb0974218be2542daa0000000000000000000000000000000000000000000000000000000000000000000003e8000007d0000000010000000000000000000000001c7d4b196cb0c7b01d743fbc6116a902379c7238000000000000000000000000558d9534cac58f743a3a9e5382f77575a2595dcb0000000000000000000000000000000000000000000000000000000000002710000000000000000000000000558d9534cac58f743a3a9e5382f77575a2595dcb00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000151caa0";
    const resp = getCctpMessageExpiryBlock(message);
    expect(resp).toBe(BigInt(22137504));
  });
});

describe("getCctpTransactionStatus", () => {
  const toChainStub: Chain = {
    id: 59141,
    cctpDomain: 11,
    cctpMessageTransmitterV2Address: "0xE737e5cEBEEBa77EFE34D4aa090756590b1CE275",
    cctpTokenMessengerV2Address: "0x8FE6B999Dc680CcFDD5Bf7EB0974218be2542DAA",
    gasLimitSurplus: 6000n,
    profitMargin: 2n,
    testnet: true,
    tokenBridgeAddress: "0x93DcAdf238932e6e6a85852caC89cBd71798F463",
    iconPath: "/images/logo/linea-sepolia.svg",
    messageServiceAddress: "0x971e727e956690b9957be6d51Ec16E73AcAC83A7",
    name: "Linea Sepolia",
    layer: ChainLayer.L2,
    nativeCurrency: {
      decimals: 18,
      name: "Linea Ether",
      symbol: "ETH",
    },
    blockExplorers: {
      default: {
        apiUrl: "https://api-sepolia.lineascan.build/api",
        name: "Etherscan",
        url: "https://sepolia.lineascan.build",
      },
    },
  };

  const cctpApiRespPending: CctpAttestationMessage = {
    attestation: "PENDING",
    message: "0x",
    eventNonce: "0xaf75c4d910592bc593c34bf6eb89937b1326ad43d9e3cf45581512efcf3e7da7",
    cctpVersion: 2,
    status: CctpAttestationMessageStatus.PENDING_CONFIRMATIONS,
  };

  // Comment out because didn't find a use yet for test, but is a valid API response
  // const cctpApiRespReady: CctpAttestationMessage = {
  //   attestation:
  //     "0x362dff8a6f0a2c55345242a652bc8bef85dd206d02660c606b34a1f89574c25a58918896870a036f3271da65a9bff30f87d65f50f212dda4a3034fb77d110f511b3d44e4f71c4cdbb53f4e1f6d3f6357b87d056722151259f00f7beaa5af3ee35d21e48a00f932ba0ea02b19f26cb9b5dcd328f491ea91cb6b9ad896f493abe9c91c",
  //   message:
  //     "0x00000001000000000000000baf75c4d910592bc593c34bf6eb89937b1326ad43d9e3cf45581512efcf3e7da70000000000000000000000008fe6b999dc680ccfdd5bf7eb0974218be2542daa0000000000000000000000008fe6b999dc680ccfdd5bf7eb0974218be2542daa0000000000000000000000000000000000000000000000000000000000000000000003e8000007d0000000010000000000000000000000001c7d4b196cb0c7b01d743fbc6116a902379c7238000000000000000000000000558d9534cac58f743a3a9e5382f77575a2595dcb000000000000000000000000000000000000000000000000000000000000c350000000000000000000000000558d9534cac58f743a3a9e5382f77575a2595dcb000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
  //   eventNonce: "0xaf75c4d910592bc593c34bf6eb89937b1326ad43d9e3cf45581512efcf3e7da7",
  //   cctpVersion: 2,
  //   status: CctpAttestationMessageStatus.COMPLETE,
  // };

  const cctpApiRespNoExpiry: CctpAttestationMessage = {
    attestation:
      "0x06207a5ca18bc3860b5c546e8a18f6032180b3792ece47747774b9de14f62b717c51f439f0263b733b223e507a1aa0e56c823633dbb6931f864598f5c428237b1ca042a1555747bf4d7dac0b4184dad34dfed9be62ce978c18d619c8c537254a2e7b8331068215677ea900e02dd00ffee67fb61a93280c0eec789a52eb03f385b51c",
    message:
      "0x00000001000000000000000b41d77498ae6f504499ff1ead8c1c2a3318d48063b8022f8215f7631153534d210000000000000000000000008fe6b999dc680ccfdd5bf7eb0974218be2542daa0000000000000000000000008fe6b999dc680ccfdd5bf7eb0974218be2542daa0000000000000000000000000000000000000000000000000000000000000000000003e8000007d0000000010000000000000000000000001c7d4b196cb0c7b01d743fbc6116a902379c7238000000000000000000000000558d9534cac58f743a3a9e5382f77575a2595dcb0000000000000000000000000000000000000000000000000000000000002710000000000000000000000000558d9534cac58f743a3a9e5382f77575a2595dcb000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000000",
    eventNonce: "0x41d77498ae6f504499ff1ead8c1c2a3318d48063b8022f8215f7631153534d21",
    cctpVersion: 2,
    status: CctpAttestationMessageStatus.COMPLETE,
  };

  // Used on Linea Sepolia
  const usedCctpNonce = "0xaf75c4d910592bc593c34bf6eb89937b1326ad43d9e3cf45581512efcf3e7da7";

  // Random nonce, should be unused
  const randomUnusedNonce = "0x97bce01d925f4152bbdc464a774bb8fcfd161557946b33930f0be0294e97eedf";

  test("should return PENDING for an empty message", async () => {
    const resp = await getCctpTransactionStatus(toChainStub, cctpApiRespPending, cctpApiRespPending.eventNonce);
    expect(resp).toBe(TransactionStatus.PENDING);
  });

  test("should return COMPLETE for a used nonce", async () => {
    const resp = await getCctpTransactionStatus(toChainStub, cctpApiRespNoExpiry, usedCctpNonce);
    expect(resp).toBe(TransactionStatus.COMPLETED);
  });

  test("should return PENDING for a truncated message", async () => {
    // Immutable creation of new object
    const corruptedCctpApiResp = { ...cctpApiRespNoExpiry };
    // Chop off last character. Last 32-bytes of message should be expirationBlock as per https://developers.circle.com/stablecoins/message-format
    corruptedCctpApiResp.message = corruptedCctpApiResp.message.slice(0, -1) as `0x${string}`;

    const resp = await getCctpTransactionStatus(toChainStub, corruptedCctpApiResp, randomUnusedNonce);
    expect(resp).toBe(TransactionStatus.PENDING);
  });

  // TODO later - Address edge case where 'parseInt("0ILLEGAL", 16) == 0',

  test("should return PENDING for a corrupted message", async () => {
    const corruptedCctpApiResp = { ...cctpApiRespNoExpiry };
    // Replace hex characters with non-hex characters
    corruptedCctpApiResp.message = (corruptedCctpApiResp.message.slice(0, -64) +
      "ILLEGALILLEGALILLEGALILLEGALILLEGALILLEGALILLEGALILLEGALILLEGALI") as `0x${string}`;

    const resp = await getCctpTransactionStatus(toChainStub, corruptedCctpApiResp, randomUnusedNonce);
    expect(resp).toBe(TransactionStatus.PENDING);
  });

  test("should return PENDING if i.) CCTP API response has pending status, ii.) message has no expiry and iii.) nonce unused", async () => {
    const stubbedCctpApiResp = { ...cctpApiRespNoExpiry };
    stubbedCctpApiResp.status = CctpAttestationMessageStatus.PENDING_CONFIRMATIONS;
    const resp = await getCctpTransactionStatus(toChainStub, stubbedCctpApiResp, randomUnusedNonce);
    expect(resp).toBe(TransactionStatus.PENDING);
  });

  test("should return READY_TO_CLAIM if i.) CCTP API response has complete status, ii.) message has no expiry and iii.) nonce unused", async () => {
    const stubbedCctpApiResp = { ...cctpApiRespNoExpiry };
    stubbedCctpApiResp.status = CctpAttestationMessageStatus.COMPLETE;

    const resp = await getCctpTransactionStatus(toChainStub, stubbedCctpApiResp, randomUnusedNonce);
    expect(resp).toBe(TransactionStatus.READY_TO_CLAIM);
  });

  test("should return PENDING and call reattest API if i.) message is expired and ii.) nonce unused", async () => {
    const expiredCctpApiResp = { ...cctpApiRespNoExpiry };
    // Put expiry block of 1
    expiredCctpApiResp.message = (expiredCctpApiResp.message.slice(0, -1) + "1") as `0x${string}`;
    // Intercept request to CCTP reattest API
    const stubbedReattestApiResp: CctpV2ReattestationApiResponse = {
      message: "message",
      nonce: randomUnusedNonce,
    };
    const interceptor = nock("https://iris-api-sandbox.circle.com")
      .post(/.*/) // Intercept all POST requests
      .reply(200, stubbedReattestApiResp);

    const resp = await getCctpTransactionStatus(toChainStub, expiredCctpApiResp, randomUnusedNonce);
    expect(resp).toBe(TransactionStatus.PENDING);
    // Assert we made a request to CCTP reattest API
    expect(interceptor.isDone()).toBe(true);
  });
});
