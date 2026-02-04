import axios from "axios";
import { readFileSync } from "fs";
import { Agent } from "https";
import forge from "node-forge";
import path from "path";
import { Hex, serializeTransaction } from "viem";

import { createLoggerMock } from "../../__tests__/helpers/factories";
import { Web3SignerClientAdapter } from "../Web3SignerClientAdapter";

// Mock getModuleDir to avoid Jest parsing issues with import.meta.url in file.ts
jest.mock("../../utils/file", () => ({
  getModuleDir: jest.fn(() => process.cwd()),
}));

jest.mock("axios", () => ({
  __esModule: true,
  default: {
    post: jest.fn(),
  },
}));

jest.mock("https", () => ({
  Agent: jest.fn(),
}));

jest.mock("fs", () => ({
  readFileSync: jest.fn(),
}));

jest.mock("node-forge", () => ({
  __esModule: true,
  default: {
    asn1: { fromDer: jest.fn() },
    pkcs12: { pkcs12FromAsn1: jest.fn() },
    pki: { oids: { certBag: "certBag" }, certificateToPem: jest.fn() },
  },
}));

jest.mock("viem", () => {
  const actual = jest.requireActual("viem");
  return {
    ...actual,
    serializeTransaction: jest.fn(),
  };
});

const AgentMock = Agent as unknown as jest.Mock;
const axiosPostMock = axios.post as jest.MockedFunction<typeof axios.post>;
const serializeTransactionMock = serializeTransaction as jest.MockedFunction<typeof serializeTransaction>;
const readFileSyncMock = readFileSync as unknown as jest.Mock;
const fromDerMock = forge.asn1.fromDer as jest.Mock;
const pkcs12FromAsn1Mock = forge.pkcs12.pkcs12FromAsn1 as jest.Mock;
const certificateToPemMock = forge.pki.certificateToPem as jest.Mock;

describe("Web3SignerClientAdapter", () => {
  const WEB3_SIGNER_URL = "https://127.0.0.1:9000";
  const WEB3_SIGNER_PUBLIC_KEY: Hex =
    "0x4a788ad6fa008beed58de6418369717d7492f37d173d70e2c26d9737e2c6eeae929452ef8602a19410844db3e200a0e73f5208fd76259a8766b73953fc3e7023";
  const WEB3_SIGNER_PUBLIC_KEY_WITHOUT_PREFIX: Hex =
    "4a788ad6fa008beed58de6418369717d7492f37d173d70e2c26d9737e2c6eeae929452ef8602a19410844db3e200a0e73f5208fd76259a8766b73953fc3e7023" as Hex;
  const KEYSTORE_PASSPHRASE = "keystore-pass";
  const TRUSTSTORE_PASSPHRASE = "trust-pass";
  const KEYSTORE_PATH = path.join(process.cwd(), "fixtures", "sequencer_client_keystore.p12");
  const TRUSTSTORE_PATH = path.join(process.cwd(), "fixtures", "web3signer_truststore.p12");
  const EXPECTED_SIGNER_ADDRESS = "0xD42E308FC964b71E18126dF469c21B0d7bcb86cC";

  const KEYSTORE_BUFFER = Buffer.from("KEYSTORE_PFX");
  const TRUSTSTORE_BINARY = "TRUSTSTORE_BINARY";
  const ASN1_STRUCT = "ASN1_STRUCT";
  const PEM_CERT = "PEM_CERT";
  const FORGE_CERTIFICATE = { subject: "CN=web3signer" };
  const AGENT_INSTANCE = { mock: "agent-instance" } as const;
  const SERIALIZED_TRANSACTION = "0x02serialized";
  const SIGNATURE_RESPONSE = "0xsigned";

  const SAMPLE_TRANSACTION = {
    type: "eip1559",
    chainId: 1337,
    nonce: 0,
    gas: BigInt(21_000),
    maxFeePerGas: BigInt(1_000_000_000),
    maxPriorityFeePerGas: BigInt(100_000_000),
    to: "0x0000000000000000000000000000000000000000",
    value: BigInt(0),
    data: "0x",
  };

  const getBagsMock = jest.fn();

  beforeEach(() => {
    jest.clearAllMocks();

    readFileSyncMock.mockImplementation((_filePath: string, options?: { encoding?: string }) => {
      if (options?.encoding === "binary") {
        return TRUSTSTORE_BINARY;
      }
      return KEYSTORE_BUFFER;
    });

    fromDerMock.mockReturnValue(ASN1_STRUCT);
    getBagsMock.mockReturnValue({ certBag: [{ cert: FORGE_CERTIFICATE }] });
    pkcs12FromAsn1Mock.mockReturnValue({ getBags: getBagsMock });
    certificateToPemMock.mockReturnValue(PEM_CERT);

    AgentMock.mockImplementation(() => AGENT_INSTANCE);
    serializeTransactionMock.mockReturnValue(SERIALIZED_TRANSACTION);
    axiosPostMock.mockResolvedValue({ data: SIGNATURE_RESPONSE } as any);
  });

  const createAdapter = (publicKey: Hex = WEB3_SIGNER_PUBLIC_KEY) =>
    new Web3SignerClientAdapter(
      createLoggerMock(),
      WEB3_SIGNER_URL,
      publicKey,
      KEYSTORE_PATH,
      KEYSTORE_PASSPHRASE,
      TRUSTSTORE_PATH,
      TRUSTSTORE_PASSPHRASE,
    );

  describe("initialization", () => {
    it("initialize HTTPS agent using keystore and truststore", () => {
      // Arrange
      // (mocks configured in beforeEach)

      // Act
      createAdapter();

      // Assert
      expect(readFileSyncMock).toHaveBeenCalledWith(TRUSTSTORE_PATH, { encoding: "binary" });
      expect(readFileSyncMock).toHaveBeenCalledWith(KEYSTORE_PATH);
      expect(fromDerMock).toHaveBeenCalledWith(TRUSTSTORE_BINARY);
      expect(pkcs12FromAsn1Mock).toHaveBeenCalledWith(ASN1_STRUCT, false, TRUSTSTORE_PASSPHRASE);
      expect(getBagsMock).toHaveBeenCalledWith({ bagType: "certBag" });
      expect(certificateToPemMock).toHaveBeenCalledWith(FORGE_CERTIFICATE);
      expect(AgentMock).toHaveBeenCalledWith({
        pfx: KEYSTORE_BUFFER,
        passphrase: KEYSTORE_PASSPHRASE,
        ca: PEM_CERT,
      });
    });

    it("throw when truststore certificate is missing", () => {
      // Arrange
      getBagsMock.mockReturnValue({});

      // Act & Assert
      expect(() => createAdapter()).toThrow("Certificate not found in P12");
      expect(certificateToPemMock).not.toHaveBeenCalled();
      expect(AgentMock).not.toHaveBeenCalled();
    });
  });

  describe("sign", () => {
    it("sign transaction via Web3Signer API", async () => {
      // Arrange
      const adapter = createAdapter();

      // Act
      const signature = await adapter.sign(SAMPLE_TRANSACTION as any);

      // Assert
      expect(serializeTransactionMock).toHaveBeenCalledWith(SAMPLE_TRANSACTION);
      expect(axiosPostMock).toHaveBeenCalledWith(
        `${WEB3_SIGNER_URL}/api/v1/eth1/sign/${WEB3_SIGNER_PUBLIC_KEY}`,
        { data: SERIALIZED_TRANSACTION },
        { httpsAgent: AGENT_INSTANCE },
      );
      expect(signature).toBe(SIGNATURE_RESPONSE);
    });
  });

  describe("getAddress", () => {
    it("derive signer address from public key with 0x prefix", () => {
      // Arrange
      const adapter = createAdapter(WEB3_SIGNER_PUBLIC_KEY);

      // Act
      const address = adapter.getAddress();

      // Assert
      expect(address).toBe(EXPECTED_SIGNER_ADDRESS);
    });

    it("derive signer address from public key without 0x prefix", () => {
      // Arrange
      const adapter = createAdapter(WEB3_SIGNER_PUBLIC_KEY_WITHOUT_PREFIX);

      // Act
      const address = adapter.getAddress();

      // Assert
      expect(address).toBe(EXPECTED_SIGNER_ADDRESS);
    });
  });
});
