import { Client, Transport, Chain, Account } from "viem";
import { getBlock } from "viem/actions";
import { linea } from "viem/chains";

import { getBlockExtraData, GetBlockExtraDataParameters } from "./getBlockExtraData";
import { generateBlock } from "../../tests/utils";

jest.mock("viem/actions", () => ({
  getBlock: jest.fn(),
}));

type MockClient = Client<Transport, Chain, Account>;

describe("getBlockExtraData", () => {
  const mockClient = (chainId: number): MockClient =>
    ({
      chain: { id: chainId },
    }) as unknown as MockClient;

  const parameters: GetBlockExtraDataParameters<"latest"> = { blockTag: "latest" };

  afterEach(() => {
    jest.clearAllMocks();
    (getBlock as jest.Mock).mockReset();
  });

  it("calls getBlock and parseBlockExtraData for Linea", async () => {
    const client = mockClient(linea.id);

    (getBlock as jest.Mock<ReturnType<typeof getBlock>>).mockResolvedValue(generateBlock());

    const result = await getBlockExtraData(client, parameters);
    expect(getBlock).toHaveBeenCalledWith(client, expect.anything());
    expect(result).toEqual({
      version: 1,
      fixedCost: 10000000000,
      variableCost: 22983624000,
      ethGasPrice: 60000000,
    });
  });
});
