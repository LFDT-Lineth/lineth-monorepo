import { expect } from "chai";
import { network } from "hardhat";
import {
  type Address,
  type Hex,
  BaseError,
  encodeAbiParameters,
  getAddress,
  hexToBigInt,
  keccak256,
  parseAbiParameters,
} from "viem";
import { generatePrivateKey, privateKeyToAddress } from "viem/accounts";

import { generateLidoUnstakePermissionlessWitness } from "./proof.js";
import { encodeData, encodeSendMessage, randomBytes32 } from "../../../../common/helpers/encoding.js";
import {
  ADDRESS_ZERO,
  BEACON_PROOF_WITNESS_TYPE,
  EMPTY_CALLDATA,
  MAX_0X2_VALIDATOR_EFFECTIVE_BALANCE_GWEI,
  ONE_ETHER,
} from "../../common/constants/index.js";
import { type TestAccount } from "../../common/helpers/index.js";

import type { ClaimMessageWithProofParams, YieldManagerInitializationData } from "./types.js";
import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

type TestYieldManager = ContractReturnType<"TestYieldManager">;
type MockYieldProvider = ContractReturnType<"MockYieldProvider">;
type TestLidoStVaultYieldProvider = ContractReturnType<"TestLidoStVaultYieldProvider">;
type MockLinethRollup = ContractReturnType<"MockLinethRollup">;
type TestLinethRollup = ContractReturnType<"TestLinethRollup">;
type MockDashboard = ContractReturnType<"MockDashboard">;
type MockVaultHub = ContractReturnType<"MockVaultHub">;
type MockSTETH = ContractReturnType<"MockSTETH">;
type SSZMerkleTree = ContractReturnType<"SSZMerkleTree">;
type TestValidatorContainerProofVerifier = ContractReturnType<"TestValidatorContainerProofVerifier">;

/** Random address, replaces ethers' `Wallet.createRandom().address`. */
export const randomAddress = (): Address => privateKeyToAddress(generatePrivateKey());

/** Impersonates `address` and returns it as a test account (ethers' `getImpersonatedSigner`). */
export const getImpersonatedAccount = async (address: Address): Promise<TestAccount> => {
  const { viem, networkHelpers } = await network.getOrCreate();
  await networkHelpers.impersonateAccount(address);
  const wallet = await viem.getWalletClient(address);
  const publicClient = await viem.getPublicClient();
  return Object.assign(wallet, { address: getAddress(address), publicClient });
};

/** Asserts that `asyncCall` reverts, whatever the reason (chai-matchers' `.to.be.reverted`). */
export const expectRevert = async (asyncCall: Promise<unknown>): Promise<void> => {
  let error: unknown;
  try {
    await asyncCall;
  } catch (e) {
    error = e;
  }
  expect(error, "Expected the call to revert, but it did not revert").to.not.equal(undefined);
  expect(
    error instanceof BaseError && /revert/i.test(error.message),
    `Expected a revert, got: ${String(error)}`,
  ).to.equal(true);
};

// TODO - Existence of this setup function means that YieldManager has invariants that withdraw cannot underflow for userFunds and userFundsInYieldProvidersTotal
// Caution - assumes it will only be used once, will not work for consecutive uses in its current form
export const fundYieldProviderForWithdrawal = async (
  testYieldManager: TestYieldManager,
  mockYieldProvider: MockYieldProvider,
  signer: TestAccount,
  withdrawAmount: bigint,
) => {
  const mockYieldProviderAddress = mockYieldProvider.address;
  const yieldManagerAddress = testYieldManager.address;
  // Funding cannot happen if withdrawal reserve in deficit
  const targetReserveAmount = await testYieldManager.read.targetWithdrawalReserveAmount();
  const l1MessageServiceAddress = await testYieldManager.read.getL1MessageService();
  await setBalance(l1MessageServiceAddress, targetReserveAmount);
  await setBalance(yieldManagerAddress, withdrawAmount);
  await testYieldManager.write.setWithdrawableValueReturnVal([mockYieldProviderAddress, withdrawAmount], {
    account: signer.account,
  });
  await testYieldManager.write.fundYieldProvider([mockYieldProviderAddress, withdrawAmount], {
    account: signer.account,
  });
};

export const incrementBalance = async (address: Address, increment: bigint) => {
  const curBalance = await getAddressBalance(address);
  await setBalance(address, curBalance + increment);
};

export const decrementBalance = async (address: Address, decrement: bigint) => {
  const curBalance = await getAddressBalance(address);
  await setBalance(address, curBalance - decrement);
};

export const setBalance = async (address: Address, balance: bigint) => {
  const { networkHelpers } = await network.getOrCreate();
  await networkHelpers.setBalance(address, balance);
};

const getAddressBalance = async (address: Address): Promise<bigint> => {
  const { viem } = await network.getOrCreate();
  const publicClient = await viem.getPublicClient();
  return publicClient.getBalance({ address });
};

export const getBalance = async (contract: { address: Address }) => {
  return await getAddressBalance(contract.address);
};

export const setWithdrawalReserveBalance = async (testYieldManager: TestYieldManager, balance: bigint) => {
  const l1MessageServiceAddress = await testYieldManager.read.getL1MessageService();
  await setBalance(l1MessageServiceAddress, balance);
};

export const setWithdrawalReserveToMinimum = async (testYieldManager: TestYieldManager) => {
  const minimumReserve = await testYieldManager.read.getEffectiveMinimumWithdrawalReserve();
  await setWithdrawalReserveBalance(testYieldManager, minimumReserve);
  return minimumReserve;
};

export const setWithdrawalReserveToTarget = async (testYieldManager: TestYieldManager) => {
  const targetReserve = await testYieldManager.read.getEffectiveTargetWithdrawalReserve();
  await setWithdrawalReserveBalance(testYieldManager, targetReserve);
  return targetReserve;
};

export const ossifyYieldProvider = async (
  yieldManager: TestYieldManager,
  yieldProviderAddress: Address,
  securityCouncil: TestAccount,
) => {
  await yieldManager.write.initiateOssification([yieldProviderAddress], { account: securityCouncil.account });
  await yieldManager.write.progressPendingOssification([yieldProviderAddress], { account: securityCouncil.account });
  expect(await yieldManager.read.isOssified([yieldProviderAddress])).to.be.true;
};

export const fundLidoStVaultYieldProvider = async (
  testYieldManager: TestYieldManager,
  yieldProvider: TestLidoStVaultYieldProvider,
  signer: TestAccount,
  withdrawAmount: bigint,
) => {
  const yieldProviderAddress = yieldProvider.address;
  const yieldManagerAddress = testYieldManager.address;
  // Funding cannot happen if withdrawal reserve in deficit
  const targetReserveAmount = await testYieldManager.read.targetWithdrawalReserveAmount();
  const l1MessageServiceAddress = await testYieldManager.read.getL1MessageService();
  await setBalance(l1MessageServiceAddress, targetReserveAmount);
  await setBalance(yieldManagerAddress, withdrawAmount);
  await testYieldManager.write.fundYieldProvider([yieldProviderAddress, withdrawAmount], { account: signer.account });
};

export const getWithdrawLSTCall = async (
  mockLinethRollup: MockLinethRollup,
  yieldManager: TestYieldManager,
  yieldProvider: TestLidoStVaultYieldProvider,
  signer: TestAccount,
  withdrawAmount: bigint,
) => {
  const { networkHelpers } = await network.getOrCreate();
  const recipient = randomAddress();
  await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, signer, withdrawAmount);

  // Add gas fees
  const l1MessageService = await yieldManager.read.L1_MESSAGE_SERVICE();
  await setBalance(l1MessageService, ONE_ETHER);
  await networkHelpers.impersonateAccount(l1MessageService);
  await mockLinethRollup.write.setWithdrawLSTAllowed([true]);

  return yieldManager.write.withdrawLST([yieldProvider.address, withdrawAmount, recipient], {
    account: l1MessageService,
  });
};

export const setupLinethRollupMessageMerkleTree = async (
  linethRollup: TestLinethRollup,
  from: Address,
  to: Address,
  value: bigint,
  data: Hex,
  securityCouncil: TestAccount,
): Promise<ClaimMessageWithProofParams> => {
  // Generate random L2 message number (not correlated with L1's nextMessageNumber)
  const messageNumber = hexToBigInt(randomBytes32());
  const expectedBytes = encodeSendMessage(from, to, 0n, value, messageNumber, data);

  const messageHash = keccak256(expectedBytes);
  const proof = Array.from({ length: 32 }, () => randomBytes32());
  const leafIndex = 0;
  const root = await linethRollup.read.generateMerkleRoot([messageHash, proof, leafIndex]);
  await linethRollup.write.addL2MerkleRoots([[root], BigInt(proof.length)], { account: securityCouncil.account });

  const claimParams: ClaimMessageWithProofParams = {
    proof,
    messageNumber: messageNumber,
    leafIndex,
    from: from,
    to: to,
    fee: 0n,
    value: value,
    feeRecipient: ADDRESS_ZERO,
    merkleRoot: root,
    data: data,
  };

  return claimParams;
};

export const incurPositiveYield = async (
  yieldManager: TestYieldManager,
  mockDashboard: MockDashboard,
  mockVaultHub: MockVaultHub,
  mockSTETH: MockSTETH,
  nativeYieldOperator: TestAccount,
  mockStakingVaultAddress: Address,
  yieldProviderAddress: Address,
  l2YieldRecipient: TestAccount,
  positiveYield: bigint,
  lstLiabilityPrincipal = 0n,
  lidoProtocolFee = 0n,
  nodeOperatorFee = 0n,
) => {
  const userFunds = await yieldManager.read.userFunds([yieldProviderAddress]);
  const mockDashboardTotalValuePrev = await mockDashboard.read.totalValue();
  const prevNegativeYield = mockDashboardTotalValuePrev < userFunds ? userFunds - mockDashboardTotalValuePrev : 0n;
  await incrementBalance(mockStakingVaultAddress, positiveYield);
  await incrementMockDashboardTotalValue(mockDashboard, positiveYield);
  const yieldProviderYieldReportedCumulativePrev = await yieldManager.read.getYieldProviderYieldReportedCumulative([
    yieldProviderAddress,
  ]);

  // Setup obligations and their withdrawals from StakingVault
  await mockDashboard.write.setAccruedFeeReturn([nodeOperatorFee]);
  await mockDashboard.write.setIsDisburseFeeWithdrawingFromVault([true]);
  await mockDashboard.write.setObligationsFeesToSettleReturn([lidoProtocolFee]);
  await mockVaultHub.write.setSettleVaultObligationAmount([lidoProtocolFee]);
  await mockVaultHub.write.setIsSettleLidoFeesWithdrawingFromVault([true]);
  await mockSTETH.write.setPooledEthBySharesRoundUpReturn([lstLiabilityPrincipal]);
  await mockSTETH.write.setSharesByPooledEthReturn([lstLiabilityPrincipal]);
  await mockDashboard.write.setLiabilitySharesReturn([lstLiabilityPrincipal]);
  await mockDashboard.write.setRebalanceVaultWithSharesWithdrawingFromVault([true]);

  // Act
  const {
    result: [newReportedYield, outstandingNegativeYield],
  } = await yieldManager.simulate.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
    account: nativeYieldOperator.address,
  });

  expect(newReportedYield).eq(
    positiveYield - lstLiabilityPrincipal - lidoProtocolFee - nodeOperatorFee - prevNegativeYield,
  );
  expect(outstandingNegativeYield).eq(0n);
  await yieldManager.write.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
    account: nativeYieldOperator.account,
  });

  // Cleanup obligation setup
  await mockDashboard.write.setAccruedFeeReturn([0n]);
  await mockDashboard.write.setObligationsFeesToSettleReturn([0n]);
  await mockVaultHub.write.setSettleVaultObligationAmount([0n]);
  await mockSTETH.write.setPooledEthBySharesRoundUpReturn([0n]);
  await mockSTETH.write.setSharesByPooledEthReturn([0n]);
  await mockDashboard.write.setLiabilitySharesReturn([0n]);
  await decrementMockDashboardTotalValue(mockDashboard, lstLiabilityPrincipal);
  await decrementMockDashboardTotalValue(mockDashboard, lidoProtocolFee);
  await decrementMockDashboardTotalValue(mockDashboard, nodeOperatorFee);

  // Asserts
  expect(await yieldManager.read.userFunds([yieldProviderAddress])).eq(userFunds + newReportedYield);
  expect(await yieldManager.read.getYieldProviderYieldReportedCumulative([yieldProviderAddress])).eq(
    yieldProviderYieldReportedCumulativePrev +
      positiveYield -
      lstLiabilityPrincipal -
      lidoProtocolFee -
      nodeOperatorFee -
      prevNegativeYield,
  );
  expect(await yieldManager.read.userFundsInYieldProvidersTotal()).eq(
    await yieldManager.read.userFunds([yieldProviderAddress]),
  );
};

export const incurNegativeYield = async (
  yieldManager: TestYieldManager,
  mockDashboard: MockDashboard,
  mockVaultHub: MockVaultHub,
  mockSTETH: MockSTETH,
  nativeYieldOperator: TestAccount,
  mockStakingVaultAddress: Address,
  yieldProviderAddress: Address,
  l2YieldRecipient: TestAccount,
  negativeYield: bigint,
  lstLiabilityPrincipal = 0n,
  lidoProtocolFee = 0n,
  nodeOperatorFee = 0n,
) => {
  await decrementBalance(mockStakingVaultAddress, negativeYield);
  await mockDashboard.write.setTotalValueReturn([(await mockDashboard.read.totalValue()) - negativeYield]);
  const userFunds = await yieldManager.read.userFunds([yieldProviderAddress]);
  const yieldProviderYieldReportedCumulativePrev = await yieldManager.read.getYieldProviderYieldReportedCumulative([
    yieldProviderAddress,
  ]);

  // Setup obligations and their withdrawals from StakingVault
  await mockDashboard.write.setAccruedFeeReturn([nodeOperatorFee]);
  await mockDashboard.write.setIsDisburseFeeWithdrawingFromVault([true]);
  await mockDashboard.write.setObligationsFeesToSettleReturn([lidoProtocolFee]);
  await mockVaultHub.write.setSettleVaultObligationAmount([lidoProtocolFee]);
  await mockVaultHub.write.setIsSettleLidoFeesWithdrawingFromVault([true]);
  await mockSTETH.write.setPooledEthBySharesRoundUpReturn([lstLiabilityPrincipal]);
  await mockSTETH.write.setSharesByPooledEthReturn([lstLiabilityPrincipal]);
  await mockDashboard.write.setLiabilitySharesReturn([lstLiabilityPrincipal]);
  await mockDashboard.write.setRebalanceVaultWithSharesWithdrawingFromVault([true]);

  // Act
  const {
    result: [newReportedYield, outstandingNegativeYield],
  } = await yieldManager.simulate.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
    account: nativeYieldOperator.address,
  });

  expect(newReportedYield).eq(0n);
  expect(outstandingNegativeYield).eq(negativeYield);
  await yieldManager.write.reportYield([yieldProviderAddress, l2YieldRecipient.address], {
    account: nativeYieldOperator.account,
  });

  // Cleanup obligation setup
  await mockDashboard.write.setAccruedFeeReturn([0n]);
  await mockDashboard.write.setObligationsFeesToSettleReturn([0n]);
  await mockVaultHub.write.setSettleVaultObligationAmount([0n]);
  await mockSTETH.write.setPooledEthBySharesRoundUpReturn([0n]);
  await mockSTETH.write.setSharesByPooledEthReturn([0n]);
  await mockDashboard.write.setLiabilitySharesReturn([0n]);
  await decrementMockDashboardTotalValue(mockDashboard, lstLiabilityPrincipal);
  await decrementMockDashboardTotalValue(mockDashboard, lidoProtocolFee);
  await decrementMockDashboardTotalValue(mockDashboard, nodeOperatorFee);

  // Asserts
  expect(await yieldManager.read.userFunds([yieldProviderAddress])).eq(userFunds);
  expect(await yieldManager.read.userFundsInYieldProvidersTotal()).eq(
    await yieldManager.read.userFunds([yieldProviderAddress]),
  );
  expect(await yieldManager.read.getYieldProviderYieldReportedCumulative([yieldProviderAddress])).eq(
    yieldProviderYieldReportedCumulativePrev,
  );
};

export const setupMaxLSTLiabilityPaymentForWithdrawal = async (
  yieldManager: TestYieldManager,
  mockDashboard: MockDashboard,
  mockVaultHub: MockVaultHub,
  mockSTETH: MockSTETH,
  yieldProviderAddress: Address,
  lstLiabilityPrincipal: bigint,
) => {
  await mockVaultHub.write.setIsVaultConnectedReturn([true]);
  await mockVaultHub.write.setIsSettleLidoFeesWithdrawingFromVault([true]);
  // Set liability principal decrement
  await mockSTETH.write.setPooledEthBySharesRoundUpReturn([
    (await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProviderAddress])) - lstLiabilityPrincipal,
  ]);
  // Set rebalance amount
  await mockSTETH.write.setSharesByPooledEthReturn([lstLiabilityPrincipal]);
  await mockDashboard.write.setLiabilitySharesReturn([lstLiabilityPrincipal]);
  await mockDashboard.write.setRebalanceVaultWithSharesWithdrawingFromVault([true]);
};

export const cleanupMaxLSTLiabilityPayment = async (
  mockDashboard: MockDashboard,
  mockVaultHub: MockVaultHub,
  mockSTETH: MockSTETH,
) => {
  // Setup obligations and their withdrawals from StakingVault
  await mockVaultHub.write.setIsSettleLidoFeesWithdrawingFromVault([true]);
  await mockSTETH.write.setPooledEthBySharesRoundUpReturn([0n]);
  await mockSTETH.write.setSharesByPooledEthReturn([0n]);
  await mockDashboard.write.setLiabilitySharesReturn([0n]);
  await mockDashboard.write.setRebalanceVaultWithSharesWithdrawingFromVault([true]);
};

export const withdrawLST = async (
  linethRollup: TestLinethRollup,
  nonAuthorizedAccount: TestAccount,
  yieldProviderAddress: Address,
  amount: bigint,
  securityCouncil: TestAccount,
) => {
  const recipientAddress = nonAuthorizedAccount.address;
  const claimParams = await setupLinethRollupMessageMerkleTree(
    linethRollup,
    recipientAddress,
    recipientAddress,
    amount,
    EMPTY_CALLDATA,
    securityCouncil,
  );
  await linethRollup.write.claimMessageWithProofAndWithdrawLST([claimParams, yieldProviderAddress], {
    account: nonAuthorizedAccount.account,
  });
};

export const executeUnstakePermissionless = async (
  sszMerkleTree: SSZMerkleTree,
  verifier: TestValidatorContainerProofVerifier,
  yieldManager: TestYieldManager,
  yieldProviderAddress: Address,
  mockStakingVaultAddress: Address,
  refundAddress: Address,
) => {
  const { eip4788Witness, pubkey, validatorIndex, slot } = await generateLidoUnstakePermissionlessWitness(
    sszMerkleTree,
    verifier,
    mockStakingVaultAddress,
    MAX_0X2_VALIDATOR_EFFECTIVE_BALANCE_GWEI,
  );

  const withdrawalParams = encodeAbiParameters(parseAbiParameters("bytes, address"), [pubkey, refundAddress]);
  const withdrawalParamsProof = encodeData([BEACON_PROOF_WITNESS_TYPE], [eip4788Witness.beaconProofWitness]);
  // Arrange - first unstake
  await yieldManager.write.unstakePermissionless(
    [yieldProviderAddress, validatorIndex, slot, withdrawalParams, withdrawalParamsProof],
    { value: 1n },
  );
};

export const incrementMockDashboardTotalValue = async (mockDashboard: MockDashboard, amount: bigint) => {
  await mockDashboard.write.setTotalValueReturn([(await mockDashboard.read.totalValue()) + amount]);
};

export const decrementMockDashboardTotalValue = async (mockDashboard: MockDashboard, amount: bigint) => {
  await mockDashboard.write.setTotalValueReturn([(await mockDashboard.read.totalValue()) - amount]);
};

export const setupLSTPrincipalDecrementForPaxMaximumPossibleLSTLiability = async (
  amount: bigint,
  yieldManager: TestYieldManager,
  yieldProviderAddress: Address,
  mockSTETH: MockSTETH,
  mockDashboard: MockDashboard,
) => {
  // Setup rebalanceShares > 0
  await mockDashboard.write.setLiabilitySharesReturn([1n]);
  await mockSTETH.write.setSharesByPooledEthReturn([1n]);
  // Setup _syncExternalLiabilitySettlement to deduct amount
  await mockSTETH.write.setPooledEthBySharesRoundUpReturn([
    (await yieldManager.read.getYieldProviderLstLiabilityPrincipal([yieldProviderAddress])) - amount,
  ]);
};

export const buildSetWithdrawalReserveParams = (
  initializationData: YieldManagerInitializationData,
  overrides: Partial<{ minPct: number; targetPct: number; minAmount: bigint; targetAmount: bigint }> = {},
) => ({
  minimumWithdrawalReservePercentageBps:
    overrides.minPct ?? initializationData.initialMinimumWithdrawalReservePercentageBps,
  targetWithdrawalReservePercentageBps:
    overrides.targetPct ?? initializationData.initialTargetWithdrawalReservePercentageBps,
  minimumWithdrawalReserveAmount: overrides.minAmount ?? initializationData.initialMinimumWithdrawalReserveAmount,
  targetWithdrawalReserveAmount: overrides.targetAmount ?? initializationData.initialTargetWithdrawalReserveAmount,
});
