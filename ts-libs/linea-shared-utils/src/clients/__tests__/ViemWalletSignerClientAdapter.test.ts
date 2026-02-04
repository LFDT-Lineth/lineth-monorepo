import { createWalletClient, http, parseTransaction, serializeSignature } from "viem";
import { privateKeyToAccount, privateKeyToAddress } from "viem/accounts";

import { createLoggerMock } from "../../__tests__/helpers/factories";
import { ILogger } from "../../logging/ILogger";
import { ViemWalletSignerClientAdapter } from "../ViemWalletSignerClientAdapter";

jest.mock("viem", () => {
  const actual = jest.requireActual("viem");
  return {
    ...actual,
    http: jest.fn(() => "mock-transport"),
    createWalletClient: jest.fn(),
    parseTransaction: jest.fn(),
    serializeSignature: jest.fn(),
  };
});

jest.mock("viem/accounts", () => ({
  privateKeyToAccount: jest.fn(),
  privateKeyToAddress: jest.fn(),
}));

describe("ViemWalletSignerClientAdapter", () => {
  // Test constants
  const TEST_PRIVATE_KEY = "0xabc" as const;
  const TEST_RPC_URL = "https://rpc.local";
  const TEST_CHAIN = { id: 111 } as any;
  const TEST_DERIVED_ACCOUNT = { address: "0xACCOUNT" } as any;
  const TEST_DERIVED_ADDRESS = "0xADDRESS" as any;
  const TEST_SERIALIZED_SIGNED_TX = "0xserialized";
  const TEST_SIGNATURE_HEX = "0xsig";
  const TEST_SIGNATURE_R = "0x1";
  const TEST_SIGNATURE_S = "0x2";
  const TEST_SIGNATURE_Y_PARITY = 1;
  const TEST_TRANSACTION_TO = "0xRecipient";
  const TEST_TRANSACTION_VALUE = 1n;
  const TEST_TRANSACTION_GAS = 21_000n;
  const TEST_EXISTING_SIGNATURE_R = "0xdead";
  const TEST_EXISTING_SIGNATURE_S = "0xbeef";
  const TEST_EXISTING_SIGNATURE_V = 27n;
  const TEST_EXISTING_SIGNATURE_Y_PARITY = 0;

  let logger: jest.Mocked<ILogger>;
  let walletSignTransaction: jest.Mock;
  let client: ViemWalletSignerClientAdapter;

  const mockedHttp = http as jest.MockedFunction<typeof http>;
  const mockedCreateWalletClient = createWalletClient as jest.MockedFunction<typeof createWalletClient>;
  const mockedParseTransaction = parseTransaction as jest.MockedFunction<typeof parseTransaction>;
  const mockedSerializeSignature = serializeSignature as jest.MockedFunction<typeof serializeSignature>;
  const mockedPrivateKeyToAccount = privateKeyToAccount as jest.MockedFunction<typeof privateKeyToAccount>;
  const mockedPrivateKeyToAddress = privateKeyToAddress as jest.MockedFunction<typeof privateKeyToAddress>;

  beforeEach(() => {
    logger = createLoggerMock({ name: "viem-wallet-signer" });
    walletSignTransaction = jest.fn();

    mockedHttp.mockReturnValue("mock-transport" as any);
    mockedPrivateKeyToAccount.mockReturnValue(TEST_DERIVED_ACCOUNT);
    mockedPrivateKeyToAddress.mockReturnValue(TEST_DERIVED_ADDRESS);
    mockedCreateWalletClient.mockReturnValue({ signTransaction: walletSignTransaction } as any);
    mockedParseTransaction.mockReset();
    mockedSerializeSignature.mockReset();

    client = new ViemWalletSignerClientAdapter(logger, TEST_RPC_URL, TEST_PRIVATE_KEY, TEST_CHAIN);
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe("constructor", () => {
    it("should construct wallet client with derived account and transport", () => {
      // Assert
      expect(mockedPrivateKeyToAccount).toHaveBeenCalledWith(TEST_PRIVATE_KEY);
      expect(mockedPrivateKeyToAddress).toHaveBeenCalledWith(TEST_PRIVATE_KEY);
      expect(mockedHttp).toHaveBeenCalledWith(TEST_RPC_URL);
      expect(mockedCreateWalletClient).toHaveBeenCalledWith({
        account: TEST_DERIVED_ACCOUNT,
        chain: TEST_CHAIN,
        transport: "mock-transport",
      });
    });
  });

  describe("sign", () => {
    it("should sign transaction and return serialized signature", async () => {
      // Arrange
      walletSignTransaction.mockResolvedValue(TEST_SERIALIZED_SIGNED_TX);
      mockedParseTransaction.mockReturnValue({
        r: TEST_SIGNATURE_R,
        s: TEST_SIGNATURE_S,
        yParity: TEST_SIGNATURE_Y_PARITY,
      } as any);
      mockedSerializeSignature.mockReturnValue(TEST_SIGNATURE_HEX);

      const tx = {
        to: TEST_TRANSACTION_TO,
        value: TEST_TRANSACTION_VALUE,
        gas: TEST_TRANSACTION_GAS,
      } as any;

      // Act
      const signature = await client.sign(tx);

      // Assert
      expect(signature).toBe(TEST_SIGNATURE_HEX);
      expect(walletSignTransaction).toHaveBeenCalledWith({
        to: TEST_TRANSACTION_TO,
        value: TEST_TRANSACTION_VALUE,
        gas: TEST_TRANSACTION_GAS,
      });
      expect(mockedParseTransaction).toHaveBeenCalledWith(TEST_SERIALIZED_SIGNED_TX);
      expect(mockedSerializeSignature).toHaveBeenCalledWith({
        r: TEST_SIGNATURE_R,
        s: TEST_SIGNATURE_S,
        yParity: TEST_SIGNATURE_Y_PARITY,
      });
    });

    it("should remove existing signature fields before signing", async () => {
      // Arrange
      walletSignTransaction.mockResolvedValue(TEST_SERIALIZED_SIGNED_TX);
      mockedParseTransaction.mockReturnValue({
        r: TEST_SIGNATURE_R,
        s: TEST_SIGNATURE_S,
        yParity: TEST_SIGNATURE_Y_PARITY,
      } as any);
      mockedSerializeSignature.mockReturnValue(TEST_SIGNATURE_HEX);

      const tx = {
        to: TEST_TRANSACTION_TO,
        value: TEST_TRANSACTION_VALUE,
        gas: TEST_TRANSACTION_GAS,
        r: TEST_EXISTING_SIGNATURE_R,
        s: TEST_EXISTING_SIGNATURE_S,
        v: TEST_EXISTING_SIGNATURE_V,
        yParity: TEST_EXISTING_SIGNATURE_Y_PARITY,
      } as any;

      // Act
      const signature = await client.sign(tx);

      // Assert
      expect(signature).toBe(TEST_SIGNATURE_HEX);
      expect(walletSignTransaction).toHaveBeenCalledWith({
        to: TEST_TRANSACTION_TO,
        value: TEST_TRANSACTION_VALUE,
        gas: TEST_TRANSACTION_GAS,
      });
    });

    it("should throw error when signature r component is missing", async () => {
      // Arrange
      walletSignTransaction.mockResolvedValue(TEST_SERIALIZED_SIGNED_TX);
      mockedParseTransaction.mockReturnValue({
        r: undefined,
        s: TEST_SIGNATURE_S,
        yParity: TEST_SIGNATURE_Y_PARITY,
      } as any);

      const tx = { nonce: 0n } as any;

      // Act & Assert
      await expect(client.sign(tx)).rejects.toThrow("sign - r, s or yParity missing");
      expect(logger.error).toHaveBeenCalledWith("sign - r, s or yParity missing");
      expect(mockedSerializeSignature).not.toHaveBeenCalled();
    });

    it("should throw error when signature s component is missing", async () => {
      // Arrange
      walletSignTransaction.mockResolvedValue(TEST_SERIALIZED_SIGNED_TX);
      mockedParseTransaction.mockReturnValue({
        r: TEST_SIGNATURE_R,
        s: undefined,
        yParity: TEST_SIGNATURE_Y_PARITY,
      } as any);

      const tx = { nonce: 0n } as any;

      // Act & Assert
      await expect(client.sign(tx)).rejects.toThrow("sign - r, s or yParity missing");
      expect(logger.error).toHaveBeenCalledWith("sign - r, s or yParity missing");
      expect(mockedSerializeSignature).not.toHaveBeenCalled();
    });

    it("should throw error when signature yParity component is missing", async () => {
      // Arrange
      walletSignTransaction.mockResolvedValue(TEST_SERIALIZED_SIGNED_TX);
      mockedParseTransaction.mockReturnValue({
        r: TEST_SIGNATURE_R,
        s: TEST_SIGNATURE_S,
        yParity: undefined,
      } as any);

      const tx = { nonce: 0n } as any;

      // Act & Assert
      await expect(client.sign(tx)).rejects.toThrow("sign - r, s or yParity missing");
      expect(logger.error).toHaveBeenCalledWith("sign - r, s or yParity missing");
      expect(mockedSerializeSignature).not.toHaveBeenCalled();
    });
  });

  describe("getAddress", () => {
    it("should return the derived address", () => {
      // Act
      const address = client.getAddress();

      // Assert
      expect(address).toBe(TEST_DERIVED_ADDRESS);
    });
  });
});
