import { describe, it, expect } from "@jest/globals";

import {
  TEST_CONTRACT_ADDRESS_1,
  TEST_CONTRACT_ADDRESS_2,
  TEST_L1_SIGNER_PRIVATE_KEY,
  TEST_L2_SIGNER_PRIVATE_KEY,
  TEST_RPC_URL,
} from "../../../../utils/testing/constants";
import { getConfig, validateEventsFiltersConfig } from "../config/utils";

const baseOptions = {
  l1Options: {
    rpcUrl: TEST_RPC_URL,
    messageServiceContractAddress: TEST_CONTRACT_ADDRESS_1,
    listener: {},
    claiming: { signer: { type: "private-key" as const, privateKey: TEST_L1_SIGNER_PRIVATE_KEY } },
  },
  l2Options: {
    rpcUrl: TEST_RPC_URL,
    messageServiceContractAddress: TEST_CONTRACT_ADDRESS_2,
    listener: {},
    claiming: { signer: { type: "private-key" as const, privateKey: TEST_L2_SIGNER_PRIVATE_KEY } },
  },
  l1L2AutoClaimEnabled: false,
  l2L1AutoClaimEnabled: false,
  databaseOptions: { type: "postgres" as const },
};

describe("PostmanApp config", () => {
  it("should parse valid config without throwing", () => {
    expect(() => getConfig(baseOptions)).not.toThrow();
  });

  it("should throw for invalid fromAddressFilter", () => {
    expect(() => validateEventsFiltersConfig({ fromAddressFilter: "0x123" })).toThrow(
      "Invalid fromAddressFilter: 0x123",
    );
  });

  it("should throw for invalid toAddressFilter", () => {
    expect(() => validateEventsFiltersConfig({ toAddressFilter: "0x123" })).toThrow("Invalid toAddressFilter: 0x123");
  });

  it("should throw for invalid calldataFunctionInterface", () => {
    expect(() =>
      validateEventsFiltersConfig({
        calldataFilter: {
          criteriaExpression: "calldata.amount > 0",
          calldataFunctionInterface: "not a valid function interface",
        },
      }),
    ).toThrow("Invalid calldataFunctionInterface");
  });
});
