import {
  AwsKmsSignerClientAdapter,
  ViemWalletSignerClientAdapter,
  Web3SignerClientAdapter,
} from "@consensys/linea-shared-utils";
import { describe, it, expect } from "@jest/globals";
import { mainnet } from "viem/chains";

import { TEST_L1_SIGNER_PRIVATE_KEY, TEST_RPC_URL } from "../../../../../utils/testing/constants";
import { TestLogger } from "../../../../../utils/testing/helpers";
import { createSignerClient } from "../createSignerClient";

import type { SignerConfig } from "../SignerConfig";

jest.mock("@consensys/linea-shared-utils", () => ({
  ViemWalletSignerClientAdapter: jest.fn().mockImplementation(() => ({ type: "viem-wallet" })),
  Web3SignerClientAdapter: jest.fn().mockImplementation(() => ({ type: "web3signer" })),
  AwsKmsSignerClientAdapter: {
    create: jest.fn().mockResolvedValue({ type: "aws-kms" }),
  },
}));

describe("createSignerClient", () => {
  const logger = new TestLogger("createSignerClient");

  afterEach(() => {
    jest.clearAllMocks();
  });

  it("creates a ViemWalletSignerClientAdapter for private-key config", async () => {
    const config: SignerConfig = {
      type: "private-key",
      privateKey: TEST_L1_SIGNER_PRIVATE_KEY,
    };

    const client = await createSignerClient(config, logger, TEST_RPC_URL, mainnet);

    expect(ViemWalletSignerClientAdapter).toHaveBeenCalledWith(logger, TEST_RPC_URL, config.privateKey, mainnet);
    expect(client).toBeDefined();
  });

  it("creates a Web3SignerClientAdapter for web3signer config without TLS", async () => {
    const config: SignerConfig = {
      type: "web3signer",
      endpoint: "http://web3signer:9000",
      publicKey: "0xaabbccdd",
    };

    const client = await createSignerClient(config, logger, TEST_RPC_URL, mainnet);

    expect(Web3SignerClientAdapter).toHaveBeenCalledWith(logger, config.endpoint, config.publicKey, "", "", "", "");
    expect(client).toBeDefined();
  });

  it("creates a Web3SignerClientAdapter for web3signer config with TLS", async () => {
    const config: SignerConfig = {
      type: "web3signer",
      endpoint: "https://web3signer:9000",
      publicKey: "0xaabbccdd",
      tls: {
        keyStorePath: "/certs/keystore.p12",
        keyStorePassword: "keystorepass",
        trustStorePath: "/certs/truststore.p12",
        trustStorePassword: "truststorepass",
      },
    };

    await createSignerClient(config, logger, TEST_RPC_URL, mainnet);

    expect(Web3SignerClientAdapter).toHaveBeenCalledWith(
      logger,
      config.endpoint,
      config.publicKey,
      config.tls!.keyStorePath,
      config.tls!.keyStorePassword,
      config.tls!.trustStorePath,
      config.tls!.trustStorePassword,
    );
  });

  it("creates an AwsKmsSignerClientAdapter for aws-kms config without region", async () => {
    const config: SignerConfig = {
      type: "aws-kms",
      kmsKeyId: "alias/linea-postman-l1",
    };

    const client = await createSignerClient(config, logger, TEST_RPC_URL, mainnet);

    expect(AwsKmsSignerClientAdapter.create).toHaveBeenCalledWith(logger, config.kmsKeyId, undefined);
    expect(client).toBeDefined();
  });

  it("creates an AwsKmsSignerClientAdapter for aws-kms config with region", async () => {
    const config: SignerConfig = {
      type: "aws-kms",
      kmsKeyId: "arn:aws:kms:eu-west-1:000000000000:key/abcd-1234",
      region: "eu-west-1",
    };

    await createSignerClient(config, logger, TEST_RPC_URL, mainnet);

    expect(AwsKmsSignerClientAdapter.create).toHaveBeenCalledWith(logger, config.kmsKeyId, { region: config.region });
  });

  it("throws with the unsupported type for an unknown signer type (runtime guard)", async () => {
    const malformedConfig = { type: "gcp-kms" };

    await expect(
      createSignerClient(malformedConfig as unknown as SignerConfig, logger, TEST_RPC_URL, mainnet),
    ).rejects.toThrow("Unsupported signer type: gcp-kms");

    expect(ViemWalletSignerClientAdapter).not.toHaveBeenCalled();
    expect(Web3SignerClientAdapter).not.toHaveBeenCalled();
    expect(AwsKmsSignerClientAdapter.create).not.toHaveBeenCalled();
  });
});
