/*
 * Copyright Consensys Software Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package net.consensys.linea.zktracer.types;

import static graphql.com.google.common.base.Preconditions.checkState;
import static net.consensys.linea.zktracer.Trace.*;
import static net.consensys.linea.zktracer.module.Util.getTxTypeAsInt;
import static net.consensys.linea.zktracer.module.hub.AccountSnapshot.canonical;
import static net.consensys.linea.zktracer.module.hub.AccountSnapshot.canonicalWithoutFrame;
import static net.consensys.linea.zktracer.types.AddressUtils.effectiveToAddress;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBoolean;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;
import static net.consensys.linea.zktracer.types.TransactionUtils.transactionHasEip1559GasSemantics;
import static org.hyperledger.besu.datatypes.TransactionType.FRONTIER;

import java.math.BigInteger;
import java.util.*;
import lombok.Getter;
import lombok.Setter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Fork;
import net.consensys.linea.zktracer.module.hub.AccountSnapshot;
import net.consensys.linea.zktracer.module.hub.ExecutionType;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.account.TimeAndExistence;
import net.consensys.linea.zktracer.module.hub.fragment.transaction.UserTransactionFragment;
import net.consensys.linea.zktracer.module.hub.section.halt.AttemptedSelfDestruct;
import net.consensys.linea.zktracer.module.hub.section.halt.EphemeralAccount;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.*;
import org.hyperledger.besu.evm.account.Account;
import org.hyperledger.besu.evm.log.Log;
import org.hyperledger.besu.evm.worldstate.WorldView;

@Getter
public class TransactionProcessingMetadata {

  final Hub hub;

  final int userTransactionNumber;
  final int relativeTransactionNumber;
  final int relativeBlockNumber;

  final Address coinbaseAddress;

  final Transaction besuTransaction;
  final long baseFee;

  final boolean isDeployment;
  int updatedRecipientAddressDeploymentNumberAtTransactionStart;
  boolean updatedRecipientAddressDeploymentStatusAtTransactionStart;
  int delegationNumberAtTransactionStart;

  @Accessors(fluent = true)
  boolean requiresEvmExecution;

  @Accessors(fluent = true)
  boolean copyTransactionCallData;

  final BigInteger initialBalance;

  final long dataCost;
  final long initCodeCost;
  final long accessListCost;
  final long delegationListCost;
  final long floorCost;
  final long floorCostPrague;

  /* g in the EYP, defined by g = TG - g0 */
  final long initiallyAvailableGas;

  final Address effectiveRecipient;

  final long effectiveGasPrice;

  /* g' in the EYP*/
  @Setter long leftoverGas = -1;
  /*  Ar in the EYP*/
  @Setter long refundCounterMax = -1;
  /* g* - g' in the EYP*/
  @Setter long refundEffective = -1;
  /* Tg - g' in the EYP*/
  @Setter long gasUsed = -1;
  /* g* in the EYP */
  @Setter long gasRefunded = -1;
  /* Tg - g* in the EYP */
  @Setter long totalGasUsed = -1;

  @Accessors(fluent = true)
  @Setter
  boolean statusCode = false;

  @Setter int hubStampTransactionEnd;

  @Setter long accumulatedGasUsedInBlock = -1;

  @Accessors(fluent = true)
  @Setter
  boolean isSenderPreWarmed = false;

  @Accessors(fluent = true)
  @Setter
  boolean isRecipientPreWarmed = false;

  @Accessors(fluent = true)
  @Setter
  boolean isCoinbasePreWarmed = false;

  @Setter List<Log> logs;

  @Setter int codeFragmentIndex = -1;

  @Setter Set<AccountSnapshot> destructedAccountsSnapshot = new HashSet<>();

  @Getter
  final Map<EphemeralAccount, List<AttemptedSelfDestruct>> unexceptionalSelfDestructMap =
      new HashMap<>();

  @Getter final Map<EphemeralAccount, Integer> effectiveSelfDestructMap = new HashMap<>();

  @Accessors(fluent = true)
  @Getter
  private final UserTransactionFragment userTransactionFragment;

  @Accessors(fluent = true)
  @Getter
  private final Map<Address, TimeAndExistence> hadCodeInitiallyMap = new HashMap<>();

  @Accessors(fluent = true)
  @Getter
  private final int type;

  @Accessors(fluent = true)
  @Getter
  private final Bytes chainId;

  @Accessors(fluent = true)
  @Getter
  private final Bytes gasPrice;

  @Accessors(fluent = true)
  @Getter
  private final Bytes maxPriorityFeePerGas;

  @Accessors(fluent = true)
  @Getter
  private final Bytes maxFeePerGas;

  @Accessors(fluent = true)
  @Getter
  private final boolean yParity;

  @Accessors(fluent = true)
  @Getter
  private final boolean replayProtection;

  @Accessors(fluent = true)
  @Getter
  private final int numberOfZeroBytesInPayload;

  @Accessors(fluent = true)
  @Getter
  private final int numberOfNonzeroBytesInPayload;

  @Accessors(fluent = true)
  @Getter
  private final int numberOfWarmedAddresses;

  @Accessors(fluent = true)
  @Getter
  private final int numberOfWarmedStorageKeys;

  @Accessors(fluent = true)
  @Getter
  private final int lengthOfDelegationList;

  @Getter @Setter private int numberOfSuccessfulDelegations = 0;
  @Getter @Setter private int numberOfSuccessfulSenderDelegations = 0;

  public TransactionProcessingMetadata(
      final Hub hub,
      final WorldView world,
      final Transaction transaction,
      final int relativeTransactionNumber,
      final int userTransactionNumber) {
    this.hub = hub;
    this.userTransactionNumber = userTransactionNumber;
    relativeBlockNumber = hub.blockStack().currentRelativeBlockNumber();
    coinbaseAddress = hub.coinbaseAddress();
    baseFee = hub.blockStack().currentBlock().baseFee().toLong();

    besuTransaction = transaction;
    this.relativeTransactionNumber = relativeTransactionNumber;

    isDeployment = transaction.getTo().isEmpty();

    initialBalance = getInitialBalance(world);

    numberOfZeroBytesInPayload = Math.toIntExact(besuTransaction.getPayloadZeroBytes());
    numberOfNonzeroBytesInPayload =
        besuTransaction.getPayload().size() - numberOfZeroBytesInPayload;

    // Note: Besu's dataCost computation contains
    // - the 21_000 transaction cost (we deduce it)
    // - the contract creation cost in case of deployment
    // - the baseline gas (gas for access lists and 7702 authorizations) is set to zero, because we
    // only consider the cost of the transaction payload
    initCodeCost =
        isDeployment ? hub.gasCalculator.initcodeCost(besuTransaction.getPayload().size()) : 0;
    dataCost = 4 * weightedByteCount();
    accessListCost =
        besuTransaction.getAccessList().map(hub.gasCalculator::accessListGasCost).orElse(0L);
    lengthOfDelegationList =
        besuTransaction.getCodeDelegationList().isPresent()
            ? besuTransaction.getCodeDelegationList().get().size()
            : 0;
    delegationListCost = (long) lengthOfDelegationList * GAS_CONST_G_PER_EMPTY_ACCOUNT_COST;
    floorCost =
        // the value below will not work in the Cancun TXN_DATA module (where it spits out 0,
        // but we still carry out the computation with the Prague value).
        hub.gasCalculator.transactionFloorCost(
            getBesuTransaction().getPayload(), numberOfZeroBytesInPayload);
    floorCostPrague = GAS_CONST_G_TRANSACTION + this.weightedByteCount() * FLOOR_TOKEN_COST;
    initiallyAvailableGas = getInitiallyAvailableGas();

    effectiveRecipient = effectiveToAddress(besuTransaction);

    effectiveGasPrice = computeEffectiveGasPrice();

    userTransactionFragment = new UserTransactionFragment(this);

    type = getTxTypeAsInt(besuTransaction.getType());
    chainId =
        besuTransaction.getChainId().isPresent()
            ? bigIntegerToBytes(besuTransaction.getChainId().get())
            : Bytes.EMPTY;
    gasPrice =
        besuTransaction.getType().supports1559FeeMarket()
            ? Bytes.EMPTY
            : bigIntegerToBytes(besuTransaction.getGasPrice().get().getAsBigInteger());
    maxPriorityFeePerGas =
        besuTransaction.getMaxPriorityFeePerGas().isPresent()
            ? bigIntegerToBytes(besuTransaction.getMaxPriorityFeePerGas().get().getAsBigInteger())
            : Bytes.EMPTY;
    maxFeePerGas =
        besuTransaction.getMaxFeePerGas().isPresent()
            ? bigIntegerToBytes(besuTransaction.getMaxFeePerGas().get().getAsBigInteger())
            : Bytes.EMPTY;
    replayProtection = besuTransaction.getChainId().isPresent();
    yParity = retrieveYParity();
    final List<AccessListEntry> accessList =
        besuTransaction.getAccessList().orElse(new ArrayList<>());
    numberOfWarmedAddresses = accessList.size();
    numberOfWarmedStorageKeys =
        accessList.stream().mapToInt(entry -> entry.storageKeys().size()).sum();
  }

  private boolean retrieveYParity() {
    // For non-legacy transactions, the Y parity is directly accessible
    if (besuTransaction.getType() != FRONTIER) {
      return bigIntegerToBoolean(besuTransaction.getYParity());
    }

    // For legacy transactions, we need to compute the Y parity based on the V value
    if (replayProtection) {
      // case chain protected, the V = 35 + 2 * chain id * Y
      return besuTransaction
          .getV()
          .equals(
              BigInteger.valueOf(PROTECTED_BASE_V_PO)
                  .add(besuTransaction.getChainId().get().multiply(BigInteger.valueOf(2))));
    }

    // case chain less, the V = 27 + Y
    return besuTransaction.getV().equals(BigInteger.valueOf(UNPROTECTED_V_PO));
  }

  public void setPreFinalisationValues(
      final long leftOverGas,
      final long refundCounterMax,
      final long accumulatedGasUsedInBlockAtStartTx) {

    this.refundCounterMax = refundCounterMax;
    setLeftoverGas(leftOverGas);
    gasUsed = computeGasUsed();
    refundEffective = computeRefundEffective();
    gasRefunded = computeRefunded();
    totalGasUsed = computeTotalGasUsed();
    accumulatedGasUsedInBlock = accumulatedGasUsedInBlockAtStartTx + totalGasUsed;
  }

  public void completeLineaTransaction(
      Hub hub,
      WorldView world,
      final boolean statusCode,
      final List<Log> logs,
      final Set<Address> selfDestructs) {
    this.statusCode = statusCode;
    hubStampTransactionEnd = hub.stamp();
    this.logs = logs;
    for (Address address : selfDestructs) {
      destructedAccountsSnapshot.add(canonical(hub, world, address));
    }

    determineSelfDestructTimeStamp();
  }

  private boolean computeCopyCallData() {
    return requiresEvmExecution && !isDeployment && !besuTransaction.getData().get().isEmpty();
  }

  /**
   * Note: if call at tracePrepareTransaction, this method could lead to invalid result as it
   * doesn't execute the authorization list.
   */
  public static boolean computeRequiresEvmExecution(
      Fork fork, WorldView world, Transaction besuTransaction) {
    if (besuTransaction.isContractCreation()) {
      return !besuTransaction.getInit().get().isEmpty();
    }

    final Optional<Account> toAccount =
        Optional.ofNullable(world.get(besuTransaction.getTo().get()));

    if (toAccount.isEmpty()) {
      return false;
    }

    final ExecutionType executionType =
        ExecutionType.getExecutionType(fork, world, besuTransaction.getTo().get());
    return executionType.pointsToExecutableCode();
  }

  public void computePostAuthorizationValues(
      Hub hub, WorldView world, Map<Address, AccountSnapshot> latestAccountSnapshots) {
    computeRealValueOfRequiresEvmExecution(hub, world, latestAccountSnapshots);
    copyTransactionCallData = computeCopyCallData();
  }

  public void computeRealValueOfRequiresEvmExecution(
      Hub hub, WorldView world, Map<Address, AccountSnapshot> latestAccountSnapshots) {

    if (besuTransaction.isContractCreation()) {
      checkState(
          besuTransaction.getInit().isPresent(),
          "Contract creation transaction should have init code");
      requiresEvmExecution = !besuTransaction.getInit().get().isEmpty();
      return;
    }

    // "message call" case
    checkState(
        besuTransaction.getTo().isPresent(),
        "Transaction isn't a deployment yet doesn't have a recipient");

    final AccountSnapshot recipientAccountSnapshot =
        latestAccountSnapshots.get(besuTransaction.getTo().get());

    ExecutionType executionType;
    if (recipientAccountSnapshot.isDelegated()) {
      checkState(
          recipientAccountSnapshot.delegationAddress().isPresent(),
          "Delegated account should have a delegation address");
      final Address delegateAddress = recipientAccountSnapshot.delegationAddress().get();

      // the delegate may or may not be among the latestAccountSnapshots
      final AccountSnapshot delegateAccountSnapshot =
          latestAccountSnapshots.containsKey(delegateAddress)
              ? latestAccountSnapshots.get(delegateAddress)
              : canonicalWithoutFrame(hub, world, delegateAddress);
      executionType =
          ExecutionType.getExecutionType(
              hub.fork, recipientAccountSnapshot, Optional.of(delegateAccountSnapshot));
    } else {
      executionType =
          ExecutionType.getExecutionType(hub.fork, recipientAccountSnapshot, Optional.empty());
    }

    requiresEvmExecution = executionType.pointsToExecutableCode();
  }

  private BigInteger getInitialBalance(WorldView world) {
    final Address sender = besuTransaction.getSender();
    return world.get(sender).getBalance().getAsBigInteger();
  }

  public long getUpfrontGasCost() {
    return dataCost
        + (isDeployment ? GAS_CONST_G_CREATE : 0)
        + (isDeployment ? initCodeCost : 0)
        + GAS_CONST_G_TRANSACTION
        + accessListCost
        + delegationListCost;
  }

  public long getInitiallyAvailableGas() {
    return getGasLimit() - getUpfrontGasCost();
  }

  private long computeRefundEffective() {
    final long upperBoundForRefunds = getGasUsed() / MAX_REFUND_QUOTIENT;
    return Math.min(upperBoundForRefunds, refundCounterMax);
  }

  private long computeEffectiveGasPrice() {
    final Transaction tx = besuTransaction;
    switch (tx.getType()) {
      case FRONTIER, ACCESS_LIST -> {
        return tx.getGasPrice().get().getAsBigInteger().longValueExact();
      }
      case EIP1559, DELEGATE_CODE -> {
        final long baseFee = this.baseFee;
        final long maxPriorityFee =
            tx.getMaxPriorityFeePerGas().get().getAsBigInteger().longValueExact();
        final long maxFeePerGas = tx.getMaxFeePerGas().get().getAsBigInteger().longValueExact();
        return Math.min(baseFee + maxPriorityFee, maxFeePerGas);
      }
      default ->
          throw new IllegalArgumentException("Transaction type not supported: " + tx.getType());
    }
  }

  public Address getSender() {
    return besuTransaction.getSender();
  }

  public boolean requiresPrewarming() {
    return requiresEvmExecution && (accessListCost != 0);
  }

  public boolean requiresAuthorizationPhase() {
    return besuTransaction.getType().supportsDelegateCode();
  }

  public boolean requiresCfiUpdate() {
    return requiresEvmExecution && isDeployment;
  }

  /* Tg - g' in the EYP*/
  public long computeGasUsed() {
    return getGasLimit() - leftoverGas;
  }

  /* g* in the EYP */
  public long computeRefunded() {

    final long leftoverGasPlusEffectiveRefund = leftoverGas + refundEffective;
    final long executionCostAfterRefunds = getGasLimit() - leftoverGasPlusEffectiveRefund;
    final long finalTransactionCost = Math.max(floorCost, executionCostAfterRefunds);

    return getGasLimit() - finalTransactionCost;
  }

  /* Tg - g* in the EYP */
  public long computeTotalGasUsed() {
    return getGasLimit() - getGasRefunded();
  }

  public long feeRateForCoinbase() {
    return switch (besuTransaction.getType()) {
      case FRONTIER, ACCESS_LIST, EIP1559, DELEGATE_CODE -> effectiveGasPrice - baseFee;
      default ->
          throw new IllegalStateException(
              "Transaction Type not supported: " + besuTransaction.getType());
    };
  }

  public Wei getCoinbaseReward() {
    return Wei.of(
        BigInteger.valueOf(totalGasUsed).multiply(BigInteger.valueOf(feeRateForCoinbase())));
  }

  public Wei getGasRefundInWei() {
    return Wei.of(BigInteger.valueOf(gasRefunded).multiply(BigInteger.valueOf(effectiveGasPrice)));
  }

  private void determineSelfDestructTimeStamp() {
    for (Map.Entry<EphemeralAccount, List<AttemptedSelfDestruct>> entry :
        unexceptionalSelfDestructMap.entrySet()) {

      final EphemeralAccount ephemeralAccount = entry.getKey();
      final List<AttemptedSelfDestruct> attemptedSelfDestructs = entry.getValue();

      // For each address, deployment number, we find selfDestructTime as
      // the time in which the first unexceptional and un-reverted SELFDESTRUCT occurs
      // Then we add this value in a new map
      for (AttemptedSelfDestruct attemptedSelfDestruct : attemptedSelfDestructs) {
        if (attemptedSelfDestruct.callFrame().wontRevert()) {
          final int selfDestructTime = attemptedSelfDestruct.hubStamp();
          effectiveSelfDestructMap.put(ephemeralAccount, selfDestructTime);
          break;
        }
      }
    }
  }

  public long weightedByteCount() {
    return 4 * numberOfNonzeroBytesInPayload + numberOfZeroBytesInPayload;
  }

  public void captureUpdatedInitialRecipientAddressDeploymentInfoAtTransactionStart(Hub hub) {
    updatedRecipientAddressDeploymentNumberAtTransactionStart =
        hub.deploymentNumberOf(effectiveRecipient);
    updatedRecipientAddressDeploymentStatusAtTransactionStart =
        hub.deploymentStatusOf(effectiveRecipient);
  }

  public Bytes getTransactionCallData() {
    return besuTransaction.getData().orElse(Bytes.EMPTY);
  }

  public boolean senderIsCoinbase() {
    return getSender().equals(coinbaseAddress);
  }

  public boolean recipientIsCoinbase() {
    return effectiveRecipient.equals(coinbaseAddress);
  }

  public boolean senderIsRecipient() {
    return getSender().equals(effectiveRecipient);
  }

  public boolean senderAddressCollision() {
    return senderIsRecipient() || senderIsCoinbase();
  }

  public boolean coinbaseAddressCollision() {
    return senderIsCoinbase() || recipientIsCoinbase();
  }

  public void updateHadCodeInitially(
      Address address, int domStamp, int subStamp, boolean hadCode, boolean seenInTxAuth) {

    final TimeAndExistence newOccurrence =
        new TimeAndExistence(domStamp, subStamp, hadCode, seenInTxAuth);

    if (hadCodeInitiallyMap.containsKey(address)) {
      final TimeAndExistence oldOccurrence = hadCodeInitiallyMap.get(address);
      if (oldOccurrence.needsUpdate(newOccurrence)) {
        hadCodeInitiallyMap.replace(address, oldOccurrence, newOccurrence);
      }
    } else {
      hadCodeInitiallyMap.put(address, newOccurrence);
    }
  }

  public long getGasLimit() {
    return besuTransaction.getGasLimit();
  }

  public boolean isMessageCall() {
    return !isDeployment;
  }

  public boolean transactionTypeHasEip1559GasSemantics() {
    return transactionHasEip1559GasSemantics(getBesuTransaction());
  }
}
