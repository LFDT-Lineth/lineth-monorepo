import { describe, it, expect } from "@jest/globals";
import { JsonRpcProvider, Wallet } from "ethers";
import { LineaSDK } from "./LineaSDK";
import { L1ClaimingService } from "./clients/ethereum";
import { LineaProvider, Provider } from "./clients/providers";
import { EthersL2MessageServiceLogClient } from "./clients/linea";
import { NETWORKS } from "./core/constants";
import { serialize } from "./core/utils";
import { TEST_L1_SIGNER_PRIVATE_KEY, TEST_L2_SIGNER_PRIVATE_KEY, TEST_RPC_URL } from "./utils/testing/constants/common";
import { generateL2MessageServiceClient, generateLineaRollupClient } from "./utils/testing/helpers";

describe("LineaSDK", () => {
  describe("getL1Contract", () => {
    it("should return LineaRollupClient in read only mode when the 'mode' option is set to 'read-only'", () => {
      const sdk = new LineaSDK({
        mode: "read-only",
        network: "linea-sepolia",
        l1RpcUrlOrProvider: TEST_RPC_URL,
        l2RpcUrlOrProvider: TEST_RPC_URL,
      });

      const lineaRollupClient = sdk.getL1Contract();
      expect(serialize(lineaRollupClient)).toStrictEqual(
        serialize(
          generateLineaRollupClient(
            new Provider(TEST_RPC_URL),
            new LineaProvider(TEST_RPC_URL),
            NETWORKS["linea-sepolia"].l1ContractAddress,
            NETWORKS["linea-sepolia"].l2ContractAddress,
            "read-only",
          ).lineaRollupClient,
        ),
      );
    });

    it("should return LineaRollupClient in read and write mode when the 'mode' option is set to 'read-write'", () => {
      const sdk = new LineaSDK({
        mode: "read-write",
        network: "linea-sepolia",
        l1SignerPrivateKeyOrWallet: TEST_L1_SIGNER_PRIVATE_KEY,
        l2SignerPrivateKeyOrWallet: TEST_L2_SIGNER_PRIVATE_KEY,
        l1RpcUrlOrProvider: TEST_RPC_URL,
        l2RpcUrlOrProvider: TEST_RPC_URL,
      });

      const lineaRollupClient = sdk.getL1Contract();
      expect(serialize(lineaRollupClient)).toStrictEqual(
        serialize(
          generateLineaRollupClient(
            new Provider(TEST_RPC_URL),
            new LineaProvider(TEST_RPC_URL),
            NETWORKS["linea-sepolia"].l1ContractAddress,
            NETWORKS["linea-sepolia"].l2ContractAddress,
            "read-write",
            new Wallet(TEST_L1_SIGNER_PRIVATE_KEY).connect(new JsonRpcProvider(TEST_RPC_URL)),
          ).lineaRollupClient,
        ),
      );
    });

    it("should return LineaRollupClient in read and write mode if l1RpcProvider is given", () => {
      const sdk = new LineaSDK({
        mode: "read-write",
        network: "linea-sepolia",
        l1SignerPrivateKeyOrWallet: TEST_L1_SIGNER_PRIVATE_KEY,
        l2SignerPrivateKeyOrWallet: TEST_L2_SIGNER_PRIVATE_KEY,
        l1RpcUrlOrProvider: TEST_RPC_URL,
        l2RpcUrlOrProvider: TEST_RPC_URL,
      });

      const lineaRollupClient = sdk.getL1Contract();
      expect(serialize(lineaRollupClient)).toStrictEqual(
        serialize(
          generateLineaRollupClient(
            new Provider(TEST_RPC_URL),
            new LineaProvider(TEST_RPC_URL),
            NETWORKS["linea-sepolia"].l1ContractAddress,
            NETWORKS["linea-sepolia"].l2ContractAddress,
            "read-write",
            new Wallet(TEST_L1_SIGNER_PRIVATE_KEY).connect(new Provider(TEST_RPC_URL)),
          ).lineaRollupClient,
        ),
      );
    });

    it("should return LineaRollupClient with the given signer if l1Wallet is given", () => {
      const wallet = new Wallet(TEST_L1_SIGNER_PRIVATE_KEY, new JsonRpcProvider(TEST_RPC_URL));
      const sdk = new LineaSDK({
        mode: "read-write",
        network: "linea-sepolia",
        l1SignerPrivateKeyOrWallet: wallet,
        l2SignerPrivateKeyOrWallet: TEST_L2_SIGNER_PRIVATE_KEY,
        l1RpcUrlOrProvider: TEST_RPC_URL,
        l2RpcUrlOrProvider: TEST_RPC_URL,
      });

      const lineaRollupClient = sdk.getL1Contract();
      expect(serialize(lineaRollupClient)).toStrictEqual(
        serialize(
          generateLineaRollupClient(
            new Provider(TEST_RPC_URL),
            new LineaProvider(TEST_RPC_URL),
            NETWORKS["linea-sepolia"].l1ContractAddress,
            NETWORKS["linea-sepolia"].l2ContractAddress,
            "read-write",
            wallet,
          ).lineaRollupClient,
        ),
      );
    });

    it("should throw an error when the 'network' option is set to 'custom' and no contract addresses have been provided to the function", () => {
      const sdk = new LineaSDK({
        mode: "read-write",
        network: "custom",
        l1SignerPrivateKeyOrWallet: TEST_L1_SIGNER_PRIVATE_KEY,
        l2SignerPrivateKeyOrWallet: TEST_L2_SIGNER_PRIVATE_KEY,
        l1RpcUrlOrProvider: TEST_RPC_URL,
        l2RpcUrlOrProvider: TEST_RPC_URL,
      });

      expect(() => sdk.getL1Contract()).toThrow("You need to provide a L1 contract address.");
    });

    it("should return LineaRollupClient with custom contract address when the network option is set to 'custom'", () => {
      const sdk = new LineaSDK({
        mode: "read-write",
        network: "custom",
        l1SignerPrivateKeyOrWallet: TEST_L1_SIGNER_PRIVATE_KEY,
        l2SignerPrivateKeyOrWallet: TEST_L2_SIGNER_PRIVATE_KEY,
        l1RpcUrlOrProvider: TEST_RPC_URL,
        l2RpcUrlOrProvider: TEST_RPC_URL,
      });

      const localL1ContractAddress = "0x0000000000000000000000000000000000000001";
      const localL2ContractAddress = "0x0000000000000000000000000000000000000002";

      const l1MessageService = sdk.getL1Contract(localL1ContractAddress, localL2ContractAddress);

      expect(serialize(l1MessageService)).toStrictEqual(
        serialize(
          generateLineaRollupClient(
            new Provider(TEST_RPC_URL),
            new LineaProvider(TEST_RPC_URL),
            localL1ContractAddress,
            localL2ContractAddress,
            "read-write",
            new Wallet(TEST_L1_SIGNER_PRIVATE_KEY).connect(new Provider(TEST_RPC_URL)),
          ).lineaRollupClient,
        ),
      );
    });
  });

  describe("getL2Contract", () => {
    it("should return L2MessageServiceClient in read only mode when the 'mode' option is set to 'read-only'", () => {
      const sdk = new LineaSDK({
        mode: "read-only",
        network: "linea-sepolia",
        l1RpcUrlOrProvider: TEST_RPC_URL,
        l2RpcUrlOrProvider: TEST_RPC_URL,
      });

      const l2MessageService = sdk.getL2Contract();
      expect(serialize(l2MessageService)).toStrictEqual(
        serialize(
          generateL2MessageServiceClient(
            new LineaProvider(TEST_RPC_URL),
            NETWORKS["linea-sepolia"].l2ContractAddress,
            "read-only",
          ).l2MessageServiceClient,
        ),
      );
    });

    it("should return L2MessageServiceClient in read and write mode when the 'mode' option is set to 'read-write'", () => {
      const sdk = new LineaSDK({
        mode: "read-write",
        network: "linea-sepolia",
        l1SignerPrivateKeyOrWallet: TEST_L1_SIGNER_PRIVATE_KEY,
        l2SignerPrivateKeyOrWallet: TEST_L2_SIGNER_PRIVATE_KEY,
        l1RpcUrlOrProvider: TEST_RPC_URL,
        l2RpcUrlOrProvider: TEST_RPC_URL,
      });

      const l2MessageService = sdk.getL2Contract();
      expect(serialize(l2MessageService)).toStrictEqual(
        serialize(
          generateL2MessageServiceClient(
            new LineaProvider(TEST_RPC_URL),
            NETWORKS["linea-sepolia"].l2ContractAddress,
            "read-write",
            new Wallet(TEST_L2_SIGNER_PRIVATE_KEY).connect(new LineaProvider(TEST_RPC_URL)),
          ).l2MessageServiceClient,
        ),
      );
    });

    it("should return L2MessageServiceClient in read and write mode if l2RpcProvider is given", () => {
      const sdk = new LineaSDK({
        mode: "read-write",
        network: "linea-sepolia",
        l1SignerPrivateKeyOrWallet: TEST_L1_SIGNER_PRIVATE_KEY,
        l2SignerPrivateKeyOrWallet: TEST_L2_SIGNER_PRIVATE_KEY,
        l1RpcUrlOrProvider: TEST_RPC_URL,
        l2RpcUrlOrProvider: TEST_RPC_URL,
      });

      const l2MessageService = sdk.getL2Contract();
      expect(serialize(l2MessageService)).toStrictEqual(
        serialize(
          generateL2MessageServiceClient(
            new LineaProvider(TEST_RPC_URL),
            NETWORKS["linea-sepolia"].l2ContractAddress,
            "read-write",
            new Wallet(TEST_L2_SIGNER_PRIVATE_KEY).connect(new LineaProvider(TEST_RPC_URL)),
          ).l2MessageServiceClient,
        ),
      );
    });

    it("should return L2MessageServiceClient with the given wallet if l2Wallet is given", () => {
      const wallet = new Wallet(TEST_L2_SIGNER_PRIVATE_KEY, new JsonRpcProvider(TEST_RPC_URL));
      const sdk = new LineaSDK({
        mode: "read-write",
        network: "linea-sepolia",
        l1SignerPrivateKeyOrWallet: TEST_L1_SIGNER_PRIVATE_KEY,
        l2SignerPrivateKeyOrWallet: wallet,
        l1RpcUrlOrProvider: TEST_RPC_URL,
        l2RpcUrlOrProvider: TEST_RPC_URL,
      });

      const l2MessageService = sdk.getL2Contract();
      expect(serialize(l2MessageService)).toStrictEqual(
        serialize(
          generateL2MessageServiceClient(
            new LineaProvider(TEST_RPC_URL),
            NETWORKS["linea-sepolia"].l2ContractAddress,
            "read-write",
            wallet,
          ).l2MessageServiceClient,
        ),
      );
    });

    it("should throw an error when the 'network' option is set to 'custom' and no local contract address has been provided to the function", () => {
      const sdk = new LineaSDK({
        mode: "read-write",
        network: "custom",
        l1SignerPrivateKeyOrWallet: TEST_L1_SIGNER_PRIVATE_KEY,
        l2SignerPrivateKeyOrWallet: TEST_L2_SIGNER_PRIVATE_KEY,
        l1RpcUrlOrProvider: TEST_RPC_URL,
        l2RpcUrlOrProvider: TEST_RPC_URL,
      });

      expect(() => sdk.getL2Contract()).toThrow("You need to provide a L2 contract address.");
    });

    it("should return L2MessageServiceContract with custom contract address when the network option is set to 'custom'", () => {
      const sdk = new LineaSDK({
        mode: "read-write",
        network: "custom",
        l1SignerPrivateKeyOrWallet: TEST_L1_SIGNER_PRIVATE_KEY,
        l2SignerPrivateKeyOrWallet: TEST_L2_SIGNER_PRIVATE_KEY,
        l1RpcUrlOrProvider: TEST_RPC_URL,
        l2RpcUrlOrProvider: TEST_RPC_URL,
      });

      const localContractAddress = "0x0000000000000000000000000000000000000001";

      const l2MessageService = sdk.getL2Contract(localContractAddress);
      expect(serialize(l2MessageService)).toStrictEqual(
        serialize(
          generateL2MessageServiceClient(
            new LineaProvider(TEST_RPC_URL),
            localContractAddress,
            "read-write",
            new Wallet(TEST_L2_SIGNER_PRIVATE_KEY).connect(new LineaProvider(TEST_RPC_URL)),
          ).l2MessageServiceClient,
        ),
      );
    });
  });

  describe("getL1ClaimingService", () => {
    it("should return L1ClaimingService", () => {
      const sdk = new LineaSDK({
        mode: "read-only",
        network: "linea-sepolia",
        l1RpcUrlOrProvider: TEST_RPC_URL,
        l2RpcUrlOrProvider: TEST_RPC_URL,
      });

      const l1ClaimingService = sdk.getL1ClaimingService();

      expect(serialize(l1ClaimingService)).toStrictEqual(
        serialize(
          new L1ClaimingService(
            generateLineaRollupClient(
              new Provider(TEST_RPC_URL),
              new LineaProvider(TEST_RPC_URL),
              NETWORKS["linea-sepolia"].l1ContractAddress,
              NETWORKS["linea-sepolia"].l2ContractAddress,
              "read-only",
            ).lineaRollupClient,
            generateL2MessageServiceClient(
              new LineaProvider(TEST_RPC_URL),
              NETWORKS["linea-sepolia"].l2ContractAddress,
              "read-only",
            ).l2MessageServiceClient,
            new EthersL2MessageServiceLogClient(
              new LineaProvider(TEST_RPC_URL).provider,
              NETWORKS["linea-sepolia"].l2ContractAddress,
            ),
            "linea-sepolia",
          ),
        ),
      );
    });
  });
});
