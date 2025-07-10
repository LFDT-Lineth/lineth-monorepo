import { walletActionsL1 } from "./walletL1";
import { Client, Transport, Chain, Account, Address, Hex } from "viem";
import { deposit } from "../actions/deposit";
import { claimOnL1 } from "../actions/claimOnL1";
import {
  TEST_ADDRESS_1,
  TEST_ADDRESS_2,
  TEST_CONTRACT_ADDRESS_1,
  TEST_CONTRACT_ADDRESS_2,
} from "../../tests/constants";

jest.mock("../actions/deposit", () => ({ deposit: jest.fn() }));
jest.mock("../actions/claimOnL1", () => ({ claimOnL1: jest.fn() }));

type MockClient = Client<Transport, Chain, Account>;

describe("walletActionsL1", () => {
  const mockClient = (chainId?: number): MockClient =>
    ({ chain: chainId ? { id: chainId } : undefined }) as unknown as MockClient;

  const client = mockClient(1);

  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe("with parameters", () => {
    const actions = walletActionsL1({
      lineaRollupAddress: TEST_CONTRACT_ADDRESS_1,
      l2MessageServiceAddress: TEST_CONTRACT_ADDRESS_2,
      l1TokenBridgeAddress: TEST_ADDRESS_1,
      l2TokenBridgeAddress: TEST_ADDRESS_2,
    })<Chain, Account>(client);

    it("delegates deposit to the action", async () => {
      const depositResult = ("0x" + "a".repeat(64)) as Hex;
      const params: Parameters<typeof actions.deposit>[0] = {
        l2Client: client,
        token: "0x0000000000000000000000000000000000000000" as Address,
        to: "0x0000000000000000000000000000000000000001" as Address,
        amount: 1000n,
      };
      (deposit as jest.Mock<ReturnType<typeof deposit>>).mockResolvedValue(depositResult);
      const result = await actions.deposit(params);
      expect(deposit).toHaveBeenCalledWith(client, {
        ...params,
        lineaRollupAddress: TEST_CONTRACT_ADDRESS_1,
        l2MessageServiceAddress: TEST_CONTRACT_ADDRESS_2,
        l1TokenBridgeAddress: TEST_ADDRESS_1,
        l2TokenBridgeAddress: TEST_ADDRESS_2,
      });
      expect(result).toBe(depositResult);
    });

    it("delegates claimOnL1 to the action", async () => {
      const claimResult = ("0x" + "b".repeat(64)) as Hex;
      const params: Parameters<typeof actions.claimOnL1>[0] = {
        from: "0x0000000000000000000000000000000000000001" as Address,
        to: "0x0000000000000000000000000000000000000002" as Address,
        fee: 1n,
        value: 2n,
        messageNonce: 3n,
        calldata: "0x" as Hex,
        messageProof: {
          proof: [],
          root: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
          leafIndex: 0,
        },
      };
      (claimOnL1 as jest.Mock<ReturnType<typeof claimOnL1>>).mockResolvedValue(claimResult);
      const result = await actions.claimOnL1(params);
      expect(claimOnL1).toHaveBeenCalledWith(client, { ...params, lineaRollupAddress: TEST_CONTRACT_ADDRESS_1 });
      expect(result).toBe(claimResult);
    });
  });

  describe("without parameters", () => {
    const actions = walletActionsL1()<Chain, Account>(client);
    it("delegates deposit to the action", async () => {
      const depositResult = ("0x" + "a".repeat(64)) as Hex;
      const params: Parameters<typeof actions.deposit>[0] = {
        l2Client: client,
        token: "0x0000000000000000000000000000000000000000" as Address,
        to: "0x0000000000000000000000000000000000000001" as Address,
        amount: 1000n,
      };
      (deposit as jest.Mock<ReturnType<typeof deposit>>).mockResolvedValue(depositResult);
      const result = await actions.deposit(params);
      expect(deposit).toHaveBeenCalledWith(client, params);
      expect(result).toBe(depositResult);
    });

    it("delegates claimOnL1 to the action", async () => {
      const claimResult = ("0x" + "b".repeat(64)) as Hex;
      const params: Parameters<typeof actions.claimOnL1>[0] = {
        from: "0x0000000000000000000000000000000000000001" as Address,
        to: "0x0000000000000000000000000000000000000002" as Address,
        fee: 1n,
        value: 2n,
        messageNonce: 3n,
        calldata: "0x" as Hex,
        messageProof: {
          proof: [],
          root: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
          leafIndex: 0,
        },
      };
      (claimOnL1 as jest.Mock<ReturnType<typeof claimOnL1>>).mockResolvedValue(claimResult);
      const result = await actions.claimOnL1(params);
      expect(claimOnL1).toHaveBeenCalledWith(client, params);
      expect(result).toBe(claimResult);
    });
  });
});
