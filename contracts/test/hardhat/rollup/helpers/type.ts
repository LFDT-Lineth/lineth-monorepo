import type { TestAccount } from "../../common/helpers/index.js";
import type {
  FinalizationData,
  ShnarfDataGenerator,
  AggregatedProofData,
  ExpectedCustomError,
} from "../../common/types.js";
import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";
import type { Address } from "viem";

// Re-export shared types for backward compatibility
export type { AggregatedProofData, ExpectedCustomError } from "../../common/types.js";

export type FinalizeContext = {
  linethRollup: ContractReturnType<"TestLinethRollup">;
  operator: TestAccount;
};

export type FinalizeProofConfig = {
  proofData: AggregatedProofData;
  blobParentShnarfIndex: number;
  shnarfDataGenerator: ShnarfDataGenerator;
  isMultiple: boolean;
};

export type FinalizeParams = {
  context: FinalizeContext;
  proofConfig: FinalizeProofConfig;
  overrides?: Partial<FinalizationData>;
};

export type FinalizeCallForwardingParams = {
  callforwarderAddress: Address;
  upgradedContract: ContractReturnType<"TestLinethRollup">;
};

export type FailedFinalizeParams = FinalizeParams & { expectedError: ExpectedCustomError };
export type SucceedFinalizeParams = FinalizeParams;
export type SucceedFinalizeParamsCallForwardingProxy = Omit<FinalizeParams, "context"> & {
  context: FinalizeCallForwardingParams;
};
