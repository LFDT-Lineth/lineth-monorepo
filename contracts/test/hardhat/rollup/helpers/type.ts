import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { TestLineaRollup } from "contracts/typechain-types";
import { Contract } from "ethers";

import { FinalizationData, ShnarfDataGenerator, AggregatedProofData, ExpectedCustomError } from "../../common/types";

// Re-export shared types for backward compatibility
export type { AggregatedProofData, ExpectedCustomError } from "../../common/types";

export type FinalizeContext = {
  lineaRollup: TestLineaRollup;
  operator: SignerWithAddress;
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
  callforwarderAddress: string;
  upgradedContract: Contract;
};

export type FailedFinalizeParams = FinalizeParams & { expectedError: ExpectedCustomError };
export type SucceedFinalizeParams = FinalizeParams;
export type SucceedFinalizeParamsCallForwardingProxy = Omit<FinalizeParams, "context"> & {
  context: FinalizeCallForwardingParams;
};
