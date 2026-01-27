import { createWalletClient, http, parseTransaction, serializeSignature } from "viem";
import { privateKeyToAccount, privateKeyToAddress } from "viem/accounts";

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

const createLogger = (): jest.Mocked<ILogger> =>
  ({
    name: "viem-wallet-signer",
    info: jest.fn(),
    error: jest.fn(),
    warn: jest.fn(),
    debug: jest.fn(),
  }) as jest.Mocked<ILogger>;

describe("ViemWalletSignerClientAdapter", () => {
  const privateKey = "0xabc" as const;
  const rpcUrl = "https://rpc.local";
  const chain = { id: 111 } as any;

  let logger: jest.Mocked<ILogger>;
  let walletSignTransaction: jest.Mock;
  let client: ViemWalletSignerClientAdapter;

  const mockedHttp = http as jest.MockedFunction<typeof http>;
  const mockedCreateWalletClient = createWalletClient as jest.MockedFunction<typeof createWalletClient>;
  const mockedParseTransaction = parseTransaction as jest.MockedFunction<typeof parseTransaction>;
  const mockedSerializeSignature = serializeSignature as jest.MockedFunction<typeof serializeSignature>;
  const mockedPrivateKeyToAccount = privateKeyToAccount as jest.MockedFunction<typeof privateKeyToAccount>;
  const mockedPrivateKeyToAddress = privateKeyToAddress as jest.MockedFunction<typeof privateKeyToAddress>;

  const derivedAccount = { address: "0xACCOUNT" } as any;
  const derivedAddress = "0xADDRESS" as any;

  beforeEach(() => {
    logger = createLogger();
    walletSignTransaction = jest.fn();
    mockedHttp.mockReturnValue("mock-transport" as any);
    mockedPrivateKeyToAccount.mockReturnValue(derivedAccount);
    mockedPrivateKeyToAddress.mockReturnValue(derivedAddress);
    mockedCreateWalletClient.mockReturnValue({ signTransaction: walletSignTransaction } as any);
    mockedParseTransaction.mockReset();
    mockedSerializeSignature.mockReset();

    client = new ViemWalletSignerClientAdapter(logger, rpcUrl, privateKey, chain);
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  it("constructs wallet client with derived account and transport", () => {
    expect(mockedPrivateKeyToAccount).toHaveBeenCalledWith(privateKey);
    expect(mockedPrivateKeyToAddress).toHaveBeenCalledWith(privateKey);
    expect(mockedHttp).toHaveBeenCalledWith(rpcUrl);
    expect(mockedCreateWalletClient).toHaveBeenCalledWith({
      account: derivedAccount,
      chain,
      transport: "mock-transport",
    });
  });

  it("signs a transaction removing existing signature fields and returns serialized signature", async () => {
    walletSignTransaction.mockResolvedValue("0xserialized");
    mockedParseTransaction.mockReturnValue({
      r: "0x1",
      s: "0x2",
      yParity: 1,
    } as any);
    mockedSerializeSignature.mockReturnValue("0xsig");

    const tx = {
      to: "0xRecipient",
      value: 1n,
      gas: 21_000n,
      r: "0xdead",
      s: "0xbeef",
      v: 27n,
      yParity: 0,
    } as any;

    const signature = await client.sign(tx);

    expect(signature).toBe("0xsig");
    expect(logger.debug).toHaveBeenNthCalledWith(1, "sign started...", { tx });
    expect(logger.debug).toHaveBeenNthCalledWith(2, "sign", { parsedTx: { r: "0x1", s: "0x2", yParity: 1 } });
    expect(walletSignTransaction).toHaveBeenCalledWith({ to: "0xRecipient", value: 1n, gas: 21_000n });
    expect(mockedParseTransaction).toHaveBeenCalledWith("0xserialized");
    expect(mockedSerializeSignature).toHaveBeenCalledWith({ r: "0x1", s: "0x2", yParity: 1 });
    expect(logger.debug).toHaveBeenCalledWith("sign completed signatureHex=0xsig");
    expect(logger.error).not.toHaveBeenCalled();
  });

  it("throws when the parsed transaction is missing signature parts", async () => {
    walletSignTransaction.mockResolvedValue("0xserialized");
    mockedParseTransaction.mockReturnValue({ r: undefined, s: "0x2", yParity: undefined } as any);

    await expect(client.sign({ nonce: 0n } as any)).rejects.toThrow("sign - r, s or yParity missing");
    expect(logger.error).toHaveBeenCalledWith("sign - r, s or yParity missing");
    expect(mockedSerializeSignature).not.toHaveBeenCalled();
  });

  it("returns the derived address", () => {
    expect(client.getAddress()).toBe(derivedAddress);
  });
});
