import path from "path";
import axios from "axios";
import { Agent } from "https";
import { Hex, serializeTransaction } from "viem";
import { readFileSync } from "fs";
import forge from "node-forge";
import { Web3SignerClientAdapter } from "../Web3SignerClientAdapter";
import { ILogger } from "../../logging/ILogger";

// Mock getModuleDir to avoid Jest parsing issues with import.meta.url in file.ts
jest.mock("../../utils/file", () => ({
  getModuleDir: jest.fn(() => process.cwd()),
}));

const agentInstance = { mock: "agent-instance" } as const;

const getBagsMock = jest.fn();
const forgeCertificate = { subject: "CN=web3signer" };

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

const createLogger = (): jest.Mocked<ILogger> =>
  ({
    name: "web3signer-client",
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
  }) as jest.Mocked<ILogger>;

describe("Web3SignerClientAdapter", () => {
  const web3SignerUrl = "https://127.0.0.1:9000";
  const web3SignerPublicKey: Hex =
    "0x4a788ad6fa008beed58de6418369717d7492f37d173d70e2c26d9737e2c6eeae929452ef8602a19410844db3e200a0e73f5208fd76259a8766b73953fc3e7023";
  const keystorePassphrase = "keystore-pass";
  const truststorePassphrase = "trust-pass";
  const keystorePath = path.join(process.cwd(), "fixtures", "sequencer_client_keystore.p12");
  const truststorePath = path.join(process.cwd(), "fixtures", "web3signer_truststore.p12");

  const keystoreBuffer = Buffer.from("KEYSTORE_PFX");
  const truststoreBinary = "TRUSTSTORE_BINARY";

  beforeEach(() => {
    jest.clearAllMocks();

    readFileSyncMock.mockImplementation((filePath: string, options?: { encoding?: string }) => {
      if (options?.encoding === "binary") {
        return truststoreBinary;
      }
      return keystoreBuffer;
    });

    fromDerMock.mockReturnValue("ASN1_STRUCT");
    getBagsMock.mockReturnValue({ certBag: [{ cert: forgeCertificate }] });
    pkcs12FromAsn1Mock.mockReturnValue({ getBags: getBagsMock });
    certificateToPemMock.mockReturnValue("PEM_CERT");

    AgentMock.mockImplementation(() => agentInstance);
    serializeTransactionMock.mockReturnValue("0x02serialized");
    axiosPostMock.mockResolvedValue({ data: "0xsigned" } as any);
  });

  const createAdapter = (logger: jest.Mocked<ILogger>) =>
    new Web3SignerClientAdapter(
      logger,
      web3SignerUrl,
      web3SignerPublicKey,
      keystorePath,
      keystorePassphrase,
      truststorePath,
      truststorePassphrase,
    );

  it("initialises the HTTPS agent using the provided keystore and truststore", () => {
    const logger = createLogger();

    createAdapter(logger);

    expect(logger.info).toHaveBeenCalledWith("Initialising HTTPS agent");
    expect(readFileSyncMock).toHaveBeenCalledWith(truststorePath, { encoding: "binary" });
    expect(readFileSyncMock).toHaveBeenCalledWith(keystorePath);

    expect(fromDerMock).toHaveBeenCalledWith(truststoreBinary);
    expect(pkcs12FromAsn1Mock).toHaveBeenCalledWith("ASN1_STRUCT", false, truststorePassphrase);
    expect(getBagsMock).toHaveBeenCalledWith({ bagType: "certBag" });
    expect(certificateToPemMock).toHaveBeenCalledWith(forgeCertificate);
    expect(logger.debug).toHaveBeenCalledWith("Loading trusted store certificate");

    expect(AgentMock).toHaveBeenCalledWith({
      pfx: keystoreBuffer,
      passphrase: keystorePassphrase,
      ca: "PEM_CERT",
    });
  });

  it("throws when the trusted store certificate is missing", () => {
    const logger = createLogger();
    getBagsMock.mockReturnValue({});

    expect(() => createAdapter(logger)).toThrow("Certificate not found in P12");
    expect(logger.info).toHaveBeenCalledWith("Initialising HTTPS agent");
    expect(certificateToPemMock).not.toHaveBeenCalled();
    expect(AgentMock).not.toHaveBeenCalled();
  });

  it("signs transactions via Web3Signer", async () => {
    const logger = createLogger();
    const adapter = createAdapter(logger);
    const tx = {
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

    const signature = await adapter.sign(tx as any);

    expect(serializeTransactionMock).toHaveBeenCalledWith(tx);
    expect(axiosPostMock).toHaveBeenCalledWith(
      `${web3SignerUrl}/api/v1/eth1/sign/${web3SignerPublicKey}`,
      { data: "0x02serialized" },
      { httpsAgent: agentInstance },
    );
    expect(logger.debug).toHaveBeenCalledWith("Signing transaction via remote Web3Signer");
    expect(logger.debug).toHaveBeenCalledWith("Signing successful signature=0xsigned");
    expect(signature).toBe("0xsigned");
  });

  it("derives the signer address from the configured public key", () => {
    const logger = createLogger();
    const adapter = createAdapter(logger);

    const address = adapter.getAddress();

    expect(address).toBe("0xD42E308FC964b71E18126dF469c21B0d7bcb86cC");
  });

  it("derives the signer address from the configured public key without 0x prefix", () => {
    const logger = createLogger();
    const web3SignerPublicKeyWithoutPrefix: Hex =
      "4a788ad6fa008beed58de6418369717d7492f37d173d70e2c26d9737e2c6eeae929452ef8602a19410844db3e200a0e73f5208fd76259a8766b73953fc3e7023" as Hex;
    const adapter = new Web3SignerClientAdapter(
      logger,
      web3SignerUrl,
      web3SignerPublicKeyWithoutPrefix,
      keystorePath,
      keystorePassphrase,
      truststorePath,
      truststorePassphrase,
    );

    const address = adapter.getAddress();

    expect(address).toBe("0xD42E308FC964b71E18126dF469c21B0d7bcb86cC");
  });
});
