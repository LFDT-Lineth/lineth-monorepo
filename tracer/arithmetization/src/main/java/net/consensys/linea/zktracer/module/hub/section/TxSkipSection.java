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

package net.consensys.linea.zktracer.module.hub.section;

import static com.google.common.base.Preconditions.checkArgument;
import static graphql.com.google.common.base.Preconditions.checkState;
import static net.consensys.linea.zktracer.module.hub.AccountSnapshot.canonical;
import static net.consensys.linea.zktracer.types.AddressUtils.isPrecompile;

import java.math.BigInteger;
import java.util.Map;
import net.consensys.linea.zktracer.module.hub.AccountSnapshot;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.TransactionProcessingType;
import net.consensys.linea.zktracer.module.hub.defer.EndTransactionDefer;
import net.consensys.linea.zktracer.module.hub.fragment.ContextFragment;
import net.consensys.linea.zktracer.module.hub.fragment.DomSubStampsSubFragment;
import net.consensys.linea.zktracer.module.hub.fragment.account.AccountFragment;
import net.consensys.linea.zktracer.module.hub.transients.Transients;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.worldstate.WorldView;

/**
 * SkippedTransaction latches data at the pre-execution of the transaction data that will be used
 * later, through a {@link EndTransactionDefer}, to generate the trace chunks required for the
 * proving of a pure transaction.
 */
public final class TxSkipSection extends TraceSection implements EndTransactionDefer {

  /**
   * + 1 TXN + 1 MISC + 1 ACC (sender gas payment and value transfer) + 1 ACC (recipient value
   * reception) + 1 ACC (delegate or recipient) + 1 ACC (coinbase)
   * -------------------------------------- + 1 CON (print ZERO_CONTEXT)
   * ======================================= = 6 rows at most
   */
  public static final short NB_ROWS_HUB_SKIP = 6;

  final TransactionProcessingMetadata txMetadata;
  final Map<Address, AccountSnapshot> accountSnapshots;

  final Address senderAddress;
  AccountSnapshot sender;
  AccountSnapshot senderNew;

  final Address recipientAddress;
  AccountSnapshot recipient;
  AccountSnapshot recipientNew;

  final Address delegateAddress;
  AccountSnapshot delegate;
  AccountSnapshot delegateNew;

  Address coinbaseAddress;
  AccountSnapshot coinbase;
  AccountSnapshot coinbaseNew;

  public TxSkipSection(Hub hub, WorldView world, Map<Address, AccountSnapshot> accountSnapshots) {
    super(hub, NB_ROWS_HUB_SKIP);
    hub.defers().scheduleForEndTransaction(this);

    final Transients transients = hub.transients();

    this.accountSnapshots = accountSnapshots;
    txMetadata = hub.txStack().current();
    senderAddress = txMetadata.getBesuTransaction().getSender();
    recipientAddress = txMetadata.getEffectiveRecipient();
    coinbaseAddress = hub.coinbaseAddress();

    // sender and recipient snapshots
    // Note: the balance may need to be corrected if [recipient == sender]
    sender = initialSnapshot(hub, world, senderAddress);
    recipient = initialSnapshot(hub, world, recipientAddress);

    // delegate or recipient snapshot
    // Note: the balance may need to be corrected if [delegate == sender] || [delegate == recipient]
    if (recipient.isDelegated()) {
      checkState(
          recipient.delegationAddress().isPresent(),
          "Recipient account is delegated but delegation address is not present");
      delegateAddress = recipient.delegationAddress().get();
      delegate = initialSnapshot(hub, world, delegateAddress);
    } else {
      delegateAddress = recipientAddress;
      delegate = recipient.deepCopy();
    }

    // arithmetization restriction
    checkArgument(
        !isPrecompile(hub.fork, recipientAddress),
        "Arithmetization restriction: recipient address is a precompile.");

    // sanity check + EIP-3607
    checkArgument(world.get(senderAddress) != null, "Sender account must exists");
    checkArgument(
        sender.accountHasEmptyCodeOrIsDelegated(),
        "Sender account must have empty code or be delegated for EIP-3607");

    // Sanity check for triggering TX_SKIP
    checkArgument(
        recipient.accountHasEmptyCodeOrIsDelegated(),
        "Recipient account must have empty code or be delegated to trigger TX_SKIP");

    // deployments are local to a transaction, every address should have deploymentStatus == false
    // at the start of every transaction
    checkArgument(
        !hub.deploymentStatusOf(senderAddress),
        "TX_SKIP: Sender address may not be under deployment");
    checkArgument(
        !hub.deploymentStatusOf(recipientAddress),
        "TX_SKIP: Recipient address may not be under deployment");

    // the updated deployment info appears in the "updated" account fragment
    if (txMetadata.isDeployment()) {
      transients.conflation().deploymentInfo().newDeploymentSansExecutionAt(recipientAddress);
    }
  }

  private boolean initialWarmth(Hub hub, Address address) {
    if (isPrecompile(hub.fork, address)) return true;

    if (accountSnapshots.containsKey(address)) {
      return accountSnapshots.get(address).isWarm();
    }

    return false;
  }

  private AccountSnapshot initialSnapshot(Hub hub, WorldView world, Address address) {

    final AccountSnapshot snapshot =
        (accountSnapshots.containsKey(address))
            ? accountSnapshots.get(address)
            : canonical(hub, world, address, initialWarmth(hub, address));
    snapshot.checkForDelegationIfAccountHasCode(hub);

    return snapshot;
  }

  public void coinbaseSnapshots(Hub hub, MessageFrame frame) {
    coinbaseAddress = frame.getMiningBeneficiary();
    coinbase =
        canonical(
            hub, frame.getWorldUpdater(), coinbaseAddress, initialWarmth(hub, coinbaseAddress));
    checkArgument(
        !hub.deploymentStatusOf(coinbaseAddress), "TX_SKIP: Coinbase address under deployment");
  }

  @Override
  public void resolveAtEndTransaction(
      Hub hub, WorldView world, Transaction tx, boolean statusCode) {

    checkArgument(statusCode, "TX_SKIP transactions should be successful");
    checkArgument(txMetadata.statusCode(), "meta data suggests an unsuccessful TX_SKIP");

    // may have to be modified in case of address collision
    senderNew = canonical(hub, world, senderAddress, initialWarmth(hub, senderAddress));
    recipientNew = canonical(hub, world, recipientAddress, initialWarmth(hub, recipientAddress));
    delegateNew = canonical(hub, world, delegateAddress, initialWarmth(hub, delegateAddress));
    coinbaseNew = canonical(hub, world, coinbaseAddress, initialWarmth(hub, coinbaseAddress));

    final Wei value = (Wei) txMetadata.getBesuTransaction().getValue();

    if (txMetadata.senderAddressCollision()) {
      final BigInteger gasUsed = BigInteger.valueOf(txMetadata.getTotalGasUsed());
      final BigInteger gasPrice = BigInteger.valueOf(txMetadata.getEffectiveGasPrice());
      final BigInteger gasCost = gasUsed.multiply(gasPrice);
      senderNew =
          sender
              .deepCopy()
              .decrementBalanceBy(value)
              .decrementBalanceBy(Wei.of(gasCost))
              .incrementNonceByOne();
    }

    if (txMetadata.senderIsRecipient()) {
      recipient = senderNew.deepCopy();
      recipientNew = recipient.deepCopy().incrementBalanceBy(value);
    } else {
      if (txMetadata.recipientIsCoinbase()) {
        recipientNew = coinbaseNew.deepCopy().decrementBalanceBy(txMetadata.getCoinbaseReward());
        recipient = recipientNew.deepCopy().decrementBalanceBy(value);
        if (txMetadata.isDeployment()) {
          recipient.decrementNonceByOne().decrementDeploymentNumberByOne();
        }
      }
    }

    if (recipientAddress.equals(delegateAddress)) {
      delegate = recipientNew.deepCopy();
      delegateNew = delegate;
    } else if (senderAddress.equals(delegateAddress)) {
      delegate = senderNew.deepCopy();
      delegateNew = delegate;
    }

    if (txMetadata.coinbaseAddressCollision()) {
      coinbase = coinbaseNew.deepCopy().decrementBalanceBy(txMetadata.getCoinbaseReward());
    }

    // "sender" account fragment
    final AccountFragment senderAccountFragment =
        hub.factories()
            .accountFragment()
            .makeWithTrm(
                sender,
                senderNew,
                senderAddress,
                DomSubStampsSubFragment.standardDomSubStamps(hub.stamp(), 1),
                TransactionProcessingType.USER);

    // "recipient" account fragment
    final AccountFragment recipientAccountFragment =
        hub.factories()
            .accountFragment()
            .makeWithTrm(
                recipient,
                recipientNew,
                recipientAddress,
                DomSubStampsSubFragment.standardDomSubStamps(hub.stamp(), 2),
                TransactionProcessingType.USER);

    // "delegate" account fragment
    final AccountFragment delegateAccountFragment =
        hub.factories()
            .accountFragment()
            .makeWithTrm(
                delegate,
                delegateNew,
                delegateAddress,
                DomSubStampsSubFragment.standardDomSubStamps(hub.stamp(), 3),
                TransactionProcessingType.USER);

    // "coinbase" account fragment
    final AccountFragment coinbaseAccountFragment =
        hub.factories()
            .accountFragment()
            .makeWithTrm(
                coinbase,
                coinbaseNew,
                coinbaseAddress,
                DomSubStampsSubFragment.standardDomSubStamps(hub.stamp(), 4),
                TransactionProcessingType.USER);

    addFragment(txMetadata.userTransactionFragment());
    addFragment(senderAccountFragment);
    addFragment(recipientAccountFragment);
    addFragment(delegateAccountFragment);
    addFragment(coinbaseAccountFragment);
    addFragment(ContextFragment.readZeroContextData(commonValues.hub));
  }
}
