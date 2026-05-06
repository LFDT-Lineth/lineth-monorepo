import { Client, Transport, Chain, Account, Address, Hex } from "viem";

import { walletActionsL2 } from "./walletL2";
import { TEST_ADDRESS_2, TEST_CONTRACT_ADDRESS_2 } from "../../tests/constants";
import { claimOnL2 } from "../actions/claimOnL2";
import { withdraw } from "../actions/withdraw";

jest.mock("../actions/withdraw", () => ({ withdraw: jest.fn() }));
jest.mock("../actions/claimOnL2", () => ({ claimOnL2: jest.fn() }));

type MockClient = Client<Transport, Chain, Account>;

describe("walletActionsL2", () => {
  const mockClient = (chainId?: number): MockClient =>
    ({ chain: chainId ? { id: chainId } : undefined }) as unknown as MockClient;

  const client = mockClient(1);

  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe("with parameters", () => {
    const actions = walletActionsL2({
      l2MessageServiceAddress: TEST_CONTRACT_ADDRESS_2,
      l2TokenBridgeAddress: TEST_ADDRESS_2,
    })<Chain, Account>(client);

    it("delegates withdraw to the action", async () => {
      const withdrawResult = ("0x" + "a".repeat(64)) as Hex;
      const params: Parameters<typeof actions.withdraw>[0] = {
        token: "0x0000000000000000000000000000000000000000" as Address,
        to: "0x0000000000000000000000000000000000000001" as Address,
        amount: 1000n,
      };
      (withdraw as jest.Mock<ReturnType<typeof withdraw>>).mockResolvedValue(withdrawResult);
      const result = await actions.withdraw(params);
      expect(withdraw).toHaveBeenCalledWith(client, {
        ...params,
        l2MessageServiceAddress: TEST_CONTRACT_ADDRESS_2,
        l2TokenBridgeAddress: TEST_ADDRESS_2,
      });
      expect(result).toBe(withdrawResult);
    });

    it("delegates claimOnL2 to the action", async () => {
      const claimResult = ("0x" + "b".repeat(64)) as Hex;
      const params: Parameters<typeof actions.claimOnL2>[0] = {
        from: "0x0000000000000000000000000000000000000001" as Address,
        to: "0x0000000000000000000000000000000000000002" as Address,
        fee: 1n,
        value: 2n,
        messageNonce: 3n,
        calldata: "0x" as Hex,
      };
      (claimOnL2 as jest.Mock<ReturnType<typeof claimOnL2>>).mockResolvedValue(claimResult);
      const result = await actions.claimOnL2(params);
      expect(claimOnL2).toHaveBeenCalledWith(client, { ...params, l2MessageServiceAddress: TEST_CONTRACT_ADDRESS_2 });
      expect(result).toBe(claimResult);
    });
  });

  describe("without parameters", () => {
    const actions = walletActionsL2()<Chain, Account>(client);

    it("delegates withdraw to the action", async () => {
      const withdrawResult = ("0x" + "a".repeat(64)) as Hex;
      const params: Parameters<typeof actions.withdraw>[0] = {
        token: "0x0000000000000000000000000000000000000000" as Address,
        to: "0x0000000000000000000000000000000000000001" as Address,
        amount: 1000n,
      };
      (withdraw as jest.Mock<ReturnType<typeof withdraw>>).mockResolvedValue(withdrawResult);
      const result = await actions.withdraw(params);
      expect(withdraw).toHaveBeenCalledWith(client, params);
      expect(result).toBe(withdrawResult);
    });

    it("delegates claimOnL2 to the action", async () => {
      const claimResult = ("0x" + "b".repeat(64)) as Hex;
      const params: Parameters<typeof actions.claimOnL2>[0] = {
        from: "0x0000000000000000000000000000000000000001" as Address,
        to: "0x0000000000000000000000000000000000000002" as Address,
        fee: 1n,
        value: 2n,
        messageNonce: 3n,
        calldata: "0x" as Hex,
      };
      (claimOnL2 as jest.Mock<ReturnType<typeof claimOnL2>>).mockResolvedValue(claimResult);
      const result = await actions.claimOnL2(params);
      expect(claimOnL2).toHaveBeenCalledWith(client, params);
      expect(result).toBe(claimResult);
    });
  });
});
