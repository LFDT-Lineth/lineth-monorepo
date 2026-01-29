import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { expect } from "chai";
import {
  MockDashboard,
  MockLineaRollup,
  MockYieldProvider,
  SSZMerkleTree,
  TestValidatorContainerProofVerifier,
  TestLidoStVaultYieldProvider,
  TestLineaRollup,
  TestYieldManager,
  YieldManager,
  MockSTETH,
  MockVaultHub,
} from "contracts/typechain-types";
import { ethers } from "hardhat";
import {
  ADDRESS_ZERO,
  BEACON_PROOF_WITNESS_TYPE,
  EMPTY_CALLDATA,
  MAX_0X2_VALIDATOR_EFFECTIVE_BALANCE_GWEI,
  ONE_ETHER,
} from "../../common/constants";
import { ClaimMessageWithProofParams, YieldManagerInitializationData } from "./types";
import { generateLidoUnstakePermissionlessWitness } from "./proof";
import { encodeSendMessage, randomBytes32 } from "../../../../common/helpers/encoding";
import { BaseContract } from "ethers";

// TODO - Existence of this setup function means that YieldManager has invariants that withdraw cannot underflow for userFunds and userFundsInYieldProvidersTotal
// Caution - assumes it will only be used once, will not work for consecutive uses in its current form
export const fundYieldProviderForWithdrawal = async (
  testYieldManager: TestYieldManager,
  mockYieldProvider: MockYieldProvider,
  signer: SignerWithAddress,
  withdrawAmount: bigint,
) => {
  const mockYieldProviderAddress = await mockYieldProvider.getAddress();
  const yieldManagerAddress = await testYieldManager.getAddress();
  // Funding cannot happen if withdrawal reserve in deficit
  const targetReserveAmount = await testYieldManager.targetWithdrawalReserveAmount();
  const l1MessageServiceAddress = await testYieldManager.getL1MessageService();
  await ethers.provider.send("hardhat_setBalance", [l1MessageServiceAddress, ethers.toBeHex(targetReserveAmount)]);
  await ethers.provider.send("hardhat_setBalance", [yieldManagerAddress, ethers.toBeHex(withdrawAmount)]);
  await testYieldManager.connect(signer).setWithdrawableValueReturnVal(mockYieldProviderAddress, withdrawAmount);
  await testYieldManager.connect(signer).fundYieldProvider(mockYieldProviderAddress, withdrawAmount);
};

export const incrementBalance = async (address: string, increment: bigint) => {
  const curBalance = await ethers.provider.getBalance(address);
  await ethers.provider.send("hardhat_setBalance", [address, ethers.toBeHex(curBalance + increment)]);
};

export const decrementBalance = async (address: string, decrement: bigint) => {
  const curBalance = await ethers.provider.getBalance(address);
  await ethers.provider.send("hardhat_setBalance", [address, ethers.toBeHex(curBalance - decrement)]);
};

export const setBalance = async (address: string, balance: bigint) => {
  await ethers.provider.send("hardhat_setBalance", [address, ethers.toBeHex(balance)]);
};

export const getBalance = async (contract: BaseContract) => {
  return await ethers.provider.getBalance(await contract.getAddress());
};

export const setWithdrawalReserveBalance = async (testYieldManager: TestYieldManager, balance: bigint) => {
  const l1MessageServiceAddress = await testYieldManager.getL1MessageService();
  await setBalance(l1MessageServiceAddress, balance);
};

export const setWithdrawalReserveToMinimum = async (testYieldManager: TestYieldManager) => {
  const minimumReserve = await testYieldManager.getEffectiveMinimumWithdrawalReserve();
  await setWithdrawalReserveBalance(testYieldManager, minimumReserve);
  return minimumReserve;
};

export const setWithdrawalReserveToTarget = async (testYieldManager: TestYieldManager) => {
  const targetReserve = await testYieldManager.getEffectiveTargetWithdrawalReserve();
  await setWithdrawalReserveBalance(testYieldManager, targetReserve);
  return targetReserve;
};

export const ossifyYieldProvider = async (
  yieldManager: TestYieldManager,
  yieldProviderAddress: string,
  securityCouncil: SignerWithAddress,
) => {
  await yieldManager.connect(securityCouncil).initiateOssification(yieldProviderAddress);
  await yieldManager.connect(securityCouncil).progressPendingOssification(yieldProviderAddress);
  expect(await yieldManager.isOssified(yieldProviderAddress)).to.be.true;
};

export const fundLidoStVaultYieldProvider = async (
  testYieldManager: TestYieldManager,
  yieldProvider: TestLidoStVaultYieldProvider,
  signer: SignerWithAddress,
  withdrawAmount: bigint,
) => {
  const yieldProviderAddress = await yieldProvider.getAddress();
  const yieldManagerAddress = await testYieldManager.getAddress();
  // Funding cannot happen if withdrawal reserve in deficit
  const targetReserveAmount = await testYieldManager.targetWithdrawalReserveAmount();
  const l1MessageServiceAddress = await testYieldManager.getL1MessageService();
  await ethers.provider.send("hardhat_setBalance", [l1MessageServiceAddress, ethers.toBeHex(targetReserveAmount)]);
  await ethers.provider.send("hardhat_setBalance", [yieldManagerAddress, ethers.toBeHex(withdrawAmount)]);
  await testYieldManager.connect(signer).fundYieldProvider(yieldProviderAddress, withdrawAmount);
};

export const getWithdrawLSTCall = async (
  mockLineaRollup: MockLineaRollup,
  yieldManager: TestYieldManager,
  yieldProvider: TestLidoStVaultYieldProvider,
  signer: SignerWithAddress,
  withdrawAmount: bigint,
) => {
  const recipient = ethers.Wallet.createRandom().address;
  await fundLidoStVaultYieldProvider(yieldManager, yieldProvider, signer, withdrawAmount);

  // Add gas fees
  const l1MessageService = await yieldManager.L1_MESSAGE_SERVICE();
  await ethers.provider.send("hardhat_setBalance", [l1MessageService, ethers.toBeHex(ONE_ETHER)]);
  const l1Signer = await ethers.getImpersonatedSigner(l1MessageService);
  await mockLineaRollup.setWithdrawLSTAllowed(true);

  return yieldManager.connect(l1Signer).withdrawLST(await yieldProvider.getAddress(), withdrawAmount, recipient);
};

export const setupLineaRollupMessageMerkleTree = async (
  lineaRollup: TestLineaRollup,
  from: string,
  to: string,
  value: bigint,
  data: string,
  securityCouncil: SignerWithAddress,
): Promise<ClaimMessageWithProofParams> => {
  // Generate random L2 message number (not correlated with L1's nextMessageNumber)
  const messageNumber = ethers.toBigInt(ethers.randomBytes(32));
  const expectedBytes = encodeSendMessage(from, to, 0n, value, messageNumber, data);

  const messageHash = ethers.keccak256(expectedBytes);
  const proof = Array.from({ length: 32 }, () => randomBytes32());
  const leafIndex = 0n;
  const root = await lineaRollup.generateMerkleRoot(messageHash, proof, leafIndex);
  await lineaRollup.connect(securityCouncil).addL2MerkleRoots([root], proof.length);

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
  nativeYieldOperator: SignerWithAddress,
  mockStakingVaultAddress: string,
  yieldProviderAddress: string,
  l2YieldRecipient: SignerWithAddress,
  positiveYield: bigint,
  lstLiabilityPrincipal = 0n,
  lidoProtocolFee = 0n,
  nodeOperatorFee = 0n,
) => {
  const userFunds = await yieldManager.userFunds(yieldProviderAddress);
  const mockDashboardTotalValuePrev = await mockDashboard.totalValue();
  const prevNegativeYield = mockDashboardTotalValuePrev < userFunds ? userFunds - mockDashboardTotalValuePrev : 0n;
  await incrementBalance(mockStakingVaultAddress, positiveYield);
  await incrementMockDashboardTotalValue(mockDashboard, positiveYield);
  const yieldProviderYieldReportedCumulativePrev =
    await yieldManager.getYieldProviderYieldReportedCumulative(yieldProviderAddress);

  // Setup obligations and their withdrawals from StakingVault
  await mockDashboard.setAccruedFeeReturn(nodeOperatorFee);
  await mockDashboard.setIsDisburseFeeWithdrawingFromVault(true);
  await mockDashboard.setObligationsFeesToSettleReturn(lidoProtocolFee);
  await mockVaultHub.setSettleVaultObligationAmount(lidoProtocolFee);
  await mockVaultHub.setIsSettleLidoFeesWithdrawingFromVault(true);
  await mockSTETH.setPooledEthBySharesRoundUpReturn(lstLiabilityPrincipal);
  await mockSTETH.setSharesByPooledEthReturn(lstLiabilityPrincipal);
  await mockDashboard.setLiabilitySharesReturn(lstLiabilityPrincipal);
  await mockDashboard.setRebalanceVaultWithSharesWithdrawingFromVault(true);

  // Act
  const [newReportedYield, outstandingNegativeYield] = await yieldManager
    .connect(nativeYieldOperator)
    .reportYield.staticCall(yieldProviderAddress, l2YieldRecipient);

  expect(newReportedYield).eq(
    positiveYield - lstLiabilityPrincipal - lidoProtocolFee - nodeOperatorFee - prevNegativeYield,
  );
  expect(outstandingNegativeYield).eq(0);
  await yieldManager.connect(nativeYieldOperator).reportYield(yieldProviderAddress, l2YieldRecipient);

  // Cleanup obligation setup
  await mockDashboard.setAccruedFeeReturn(0n);
  await mockDashboard.setObligationsFeesToSettleReturn(0n);
  await mockVaultHub.setSettleVaultObligationAmount(0n);
  await mockSTETH.setPooledEthBySharesRoundUpReturn(0n);
  await mockSTETH.setSharesByPooledEthReturn(0n);
  await mockDashboard.setLiabilitySharesReturn(0n);
  await decrementMockDashboardTotalValue(mockDashboard, lstLiabilityPrincipal);
  await decrementMockDashboardTotalValue(mockDashboard, lidoProtocolFee);
  await decrementMockDashboardTotalValue(mockDashboard, nodeOperatorFee);

  // Asserts
  expect(await yieldManager.userFunds(yieldProviderAddress)).eq(userFunds + newReportedYield);
  expect(await yieldManager.getYieldProviderYieldReportedCumulative(yieldProviderAddress)).eq(
    yieldProviderYieldReportedCumulativePrev +
      positiveYield -
      lstLiabilityPrincipal -
      lidoProtocolFee -
      nodeOperatorFee -
      prevNegativeYield,
  );
  expect(await yieldManager.userFundsInYieldProvidersTotal()).eq(await yieldManager.userFunds(yieldProviderAddress));
};

export const incurNegativeYield = async (
  yieldManager: TestYieldManager,
  mockDashboard: MockDashboard,
  mockVaultHub: MockVaultHub,
  mockSTETH: MockSTETH,
  nativeYieldOperator: SignerWithAddress,
  mockStakingVaultAddress: string,
  yieldProviderAddress: string,
  l2YieldRecipient: SignerWithAddress,
  negativeYield: bigint,
  lstLiabilityPrincipal = 0n,
  lidoProtocolFee = 0n,
  nodeOperatorFee = 0n,
) => {
  await decrementBalance(mockStakingVaultAddress, negativeYield);
  await mockDashboard.setTotalValueReturn((await mockDashboard.totalValue()) - negativeYield);
  const userFunds = await yieldManager.userFunds(yieldProviderAddress);
  const yieldProviderYieldReportedCumulativePrev =
    await yieldManager.getYieldProviderYieldReportedCumulative(yieldProviderAddress);

  // Setup obligations and their withdrawals from StakingVault
  await mockDashboard.setAccruedFeeReturn(nodeOperatorFee);
  await mockDashboard.setIsDisburseFeeWithdrawingFromVault(true);
  await mockDashboard.setObligationsFeesToSettleReturn(lidoProtocolFee);
  await mockVaultHub.setSettleVaultObligationAmount(lidoProtocolFee);
  await mockVaultHub.setIsSettleLidoFeesWithdrawingFromVault(true);
  await mockSTETH.setPooledEthBySharesRoundUpReturn(lstLiabilityPrincipal);
  await mockSTETH.setSharesByPooledEthReturn(lstLiabilityPrincipal);
  await mockDashboard.setLiabilitySharesReturn(lstLiabilityPrincipal);
  await mockDashboard.setRebalanceVaultWithSharesWithdrawingFromVault(true);

  // Act
  const [newReportedYield, outstandingNegativeYield] = await yieldManager
    .connect(nativeYieldOperator)
    .reportYield.staticCall(yieldProviderAddress, l2YieldRecipient);

  expect(newReportedYield).eq(0n);
  expect(outstandingNegativeYield).eq(negativeYield);
  await yieldManager.connect(nativeYieldOperator).reportYield(yieldProviderAddress, l2YieldRecipient);

  // Cleanup obligation setup
  await mockDashboard.setAccruedFeeReturn(0n);
  await mockDashboard.setObligationsFeesToSettleReturn(0n);
  await mockVaultHub.setSettleVaultObligationAmount(0n);
  await mockSTETH.setPooledEthBySharesRoundUpReturn(0n);
  await mockSTETH.setSharesByPooledEthReturn(0n);
  await mockDashboard.setLiabilitySharesReturn(0n);
  await decrementMockDashboardTotalValue(mockDashboard, lstLiabilityPrincipal);
  await decrementMockDashboardTotalValue(mockDashboard, lidoProtocolFee);
  await decrementMockDashboardTotalValue(mockDashboard, nodeOperatorFee);

  // Asserts
  expect(await yieldManager.userFunds(yieldProviderAddress)).eq(userFunds);
  expect(await yieldManager.userFundsInYieldProvidersTotal()).eq(await yieldManager.userFunds(yieldProviderAddress));
  expect(await yieldManager.getYieldProviderYieldReportedCumulative(yieldProviderAddress)).eq(
    yieldProviderYieldReportedCumulativePrev,
  );
};

export const setupMaxLSTLiabilityPaymentForWithdrawal = async (
  yieldManager: TestYieldManager,
  mockDashboard: MockDashboard,
  mockVaultHub: MockVaultHub,
  mockSTETH: MockSTETH,
  yieldProviderAddress: string,
  lstLiabilityPrincipal: bigint,
) => {
  await mockVaultHub.setIsVaultConnectedReturn(true);
  await mockVaultHub.setIsSettleLidoFeesWithdrawingFromVault(true);
  // Set liability principal decrement
  await mockSTETH.setPooledEthBySharesRoundUpReturn(
    (await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProviderAddress)) - lstLiabilityPrincipal,
  );
  // Set rebalance amount
  await mockSTETH.setSharesByPooledEthReturn(lstLiabilityPrincipal);
  await mockDashboard.setLiabilitySharesReturn(lstLiabilityPrincipal);
  await mockDashboard.setRebalanceVaultWithSharesWithdrawingFromVault(true);
};

export const cleanupMaxLSTLiabilityPayment = async (
  mockDashboard: MockDashboard,
  mockVaultHub: MockVaultHub,
  mockSTETH: MockSTETH,
) => {
  // Setup obligations and their withdrawals from StakingVault
  await mockVaultHub.setIsSettleLidoFeesWithdrawingFromVault(true);
  await mockSTETH.setPooledEthBySharesRoundUpReturn(0n);
  await mockSTETH.setSharesByPooledEthReturn(0n);
  await mockDashboard.setLiabilitySharesReturn(0n);
  await mockDashboard.setRebalanceVaultWithSharesWithdrawingFromVault(true);
};

export const withdrawLST = async (
  lineaRollup: TestLineaRollup,
  nonAuthorizedAccount: SignerWithAddress,
  yieldProviderAddress: string,
  amount: bigint,
  securityCouncil: SignerWithAddress,
) => {
  const recipientAddress = await nonAuthorizedAccount.getAddress();
  const claimParams = await setupLineaRollupMessageMerkleTree(
    lineaRollup,
    recipientAddress,
    recipientAddress,
    amount,
    EMPTY_CALLDATA,
    securityCouncil,
  );
  await lineaRollup
    .connect(nonAuthorizedAccount)
    .claimMessageWithProofAndWithdrawLST(claimParams, yieldProviderAddress);
};

export const executeUnstakePermissionless = async (
  sszMerkleTree: SSZMerkleTree,
  verifier: TestValidatorContainerProofVerifier,
  yieldManager: YieldManager,
  yieldProviderAddress: string,
  mockStakingVaultAddress: string,
  refundAddress: string,
) => {
  const { eip4788Witness, pubkey, validatorIndex, slot } = await generateLidoUnstakePermissionlessWitness(
    sszMerkleTree,
    verifier,
    mockStakingVaultAddress,
    MAX_0X2_VALIDATOR_EFFECTIVE_BALANCE_GWEI,
  );

  const withdrawalParams = ethers.AbiCoder.defaultAbiCoder().encode(["bytes", "address"], [pubkey, refundAddress]);
  const withdrawalParamsProof = ethers.AbiCoder.defaultAbiCoder().encode(
    [BEACON_PROOF_WITNESS_TYPE],
    [eip4788Witness.beaconProofWitness],
  );
  // Arrange - first unstake
  await yieldManager.unstakePermissionless(
    yieldProviderAddress,
    validatorIndex,
    slot,
    withdrawalParams,
    withdrawalParamsProof,
    { value: 1n },
  );
};

export const incrementMockDashboardTotalValue = async (mockDashboard: MockDashboard, amount: bigint) => {
  await mockDashboard.setTotalValueReturn((await mockDashboard.totalValue()) + amount);
};

export const decrementMockDashboardTotalValue = async (mockDashboard: MockDashboard, amount: bigint) => {
  await mockDashboard.setTotalValueReturn((await mockDashboard.totalValue()) - amount);
};

export const setupLSTPrincipalDecrementForPaxMaximumPossibleLSTLiability = async (
  amount: bigint,
  yieldManager: TestYieldManager,
  yieldProviderAddress: string,
  mockSTETH: MockSTETH,
  mockDashboard: MockDashboard,
) => {
  // Setup rebalanceShares > 0
  await mockDashboard.setLiabilitySharesReturn(1);
  await mockSTETH.setSharesByPooledEthReturn(1);
  // Setup _syncExternalLiabilitySettlement to deduct amount
  await mockSTETH.setPooledEthBySharesRoundUpReturn(
    (await yieldManager.getYieldProviderLstLiabilityPrincipal(yieldProviderAddress)) - amount,
  );
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
