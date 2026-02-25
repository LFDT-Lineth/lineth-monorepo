/*
 * Copyright Consensys Software Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use hub file except in compliance with
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

import static com.google.common.base.Preconditions.checkState;
import static net.consensys.linea.zktracer.module.hub.AccountSnapshot.canonicalWithoutFrame;
import static net.consensys.linea.zktracer.module.hub.HubProcessingPhase.TX_EXEC;

import java.util.Map;
import java.util.Optional;
import lombok.Getter;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.zktracer.module.hub.AccountSnapshot;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.TransactionProcessingType;
import net.consensys.linea.zktracer.module.hub.defer.EndTransactionDefer;
import net.consensys.linea.zktracer.module.hub.fragment.ContextFragment;
import net.consensys.linea.zktracer.module.hub.fragment.DomSubStampsSubFragment;
import net.consensys.linea.zktracer.module.hub.fragment.account.AccountFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.ImcFragment;
import net.consensys.linea.zktracer.types.Bytecode;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.evm.worldstate.WorldView;

@Slf4j
public final class TxInitializationSection extends TraceSection implements EndTransactionDefer {

  /**
   * + 1 TXN + 1 MISC + 1 ACC (coinbase) + 1 ACC (sender gas payment) + 1 ACC (sender value
   * transfer) + 1 ACC (recipient value reception) + 1 ACC (delegate or recipient)
   * -------------------------------------- + 1 ACC (undoing sender value transfer) + 1 ACC (undoing
   * sender value transfer) -------------------------------------- + 1 CON (intialization of root
   * frame) ======================================= = 10 rows at most
   */
  public static final short NB_ROWS_HUB_INIT = 10;

  @Getter private final int hubStamp;
  private final WorldView world;
  final AccountFragment.AccountFragmentFactory accountFragmentFactory;
  final Map<Address, AccountSnapshot> latestAccountSnapshots;

  private final ImcFragment miscFragment;

  public final AccountFragment coinbaseWarmingAccountFragment;
  @Getter private final AccountSnapshot coinbase;
  @Getter private final AccountSnapshot coinbaseNew;

  private final AccountFragment gasPaymentAccountFragment;
  @Getter private final AccountSnapshot senderGasPayment;
  @Getter private final AccountSnapshot senderGasPaymentNew;

  private final AccountFragment valueSendingAccountFragment;
  @Getter private final AccountSnapshot senderValueTransfer;
  @Getter private final AccountSnapshot senderValueTransferNew;

  private final AccountFragment valueReceptionAccountFragment;
  @Getter private final AccountSnapshot recipientValueReception;
  @Getter private final AccountSnapshot recipientValueReceptionNew;

  // the only difference between delegateNew and delegate
  // is that delegateNew does not check for deletation
  // TODO: this constraint that outside of TX_AUTH one should not
  //  check for delegation ... is a little annoying given that we
  //  like to create account snapshots by .deepCopy()
  //  I think the correct approach is to impose on the contrary that
  //  outside of TX_AUTH-rows one should impose maybe equality between
  //  the current check and the new check bits ... I don't believe this
  //  would interfere with anything (in particular deployments or so)
  private final AccountFragment delegateAccountFragment;
  @Getter private final AccountSnapshot delegateOrRecipient;
  @Getter private final AccountSnapshot delegateOrRecipientNew;

  @Getter private AccountSnapshot senderUndoingValueTransfer;
  @Getter private AccountSnapshot senderUndoingValueTransferNew;

  @Getter private AccountSnapshot recipientUndoingValueReception;
  @Getter private AccountSnapshot recipientUndoingValueReceptionNew;

  @Getter private final ContextFragment initializationContextFragment;

  /** This is used to generate the Dom / Sub offset */
  private int domSubOffset = 2;

  public TxInitializationSection(
      Hub hub, WorldView world, Map<Address, AccountSnapshot> initialAccountSnapshots) {
    super(hub, NB_ROWS_HUB_INIT);
    hub.defers().scheduleForEndTransaction(this);

    this.world = world;
    hubStamp = hub.stamp();
    accountFragmentFactory = hub.factories().accountFragment();
    latestAccountSnapshots = initialAccountSnapshots;

    hub.txStack().setInitializationSection(this);

    final TransactionProcessingMetadata tx = hub.txStack().current();
    final Address coinbaseAddress = hub.coinbaseAddress();
    final Address senderAddress = tx.getSender();
    final Address recipientAddress = tx.getEffectiveRecipient();
    Optional<Address> delegateAddress =
        latestAccountSnapshots.get(recipientAddress).delegationAddress();
    final Address delegateOrRecipientAddress = delegateAddress.orElse(recipientAddress);

    coinbase =
        deepCopyAndMaybeCheckForDelegation(hub, coinbaseAddress, "coinbase [turn on warmth]", false)
            .dontCheckForDelegation(hub);
    coinbaseNew = coinbase.deepCopy().turnOnWarmth().dontCheckForDelegation(hub);
    latestAccountSnapshots.put(coinbaseAddress, coinbaseNew);

    if (!latestAccountSnapshots.containsKey(coinbaseAddress)) {
      checkState(
          tx.isCoinbasePreWarmed() == coinbase.isWarm(),
          "If the coinbase address is not present in the latest account snapshots map, it should be warm if and only if it is a precompile address");
    }

    final Wei transactionGasPrice = Wei.of(tx.getEffectiveGasPrice());
    final Wei gasCost = transactionGasPrice.multiply(tx.getBesuTransaction().getGasLimit());

    senderGasPayment =
        deepCopyAndMaybeCheckForDelegation(hub, senderAddress, "sender [gas payment]", true);
    senderGasPaymentNew =
        senderGasPayment
            .deepCopy()
            .decrementBalanceBy(gasCost)
            .turnOnWarmth()
            .incrementNonceByOne()
            .dontCheckForDelegation(hub);
    latestAccountSnapshots.put(senderAddress, senderGasPaymentNew);

    final Wei value = (Wei) tx.getBesuTransaction().getValue();

    senderValueTransfer =
        deepCopyAndMaybeCheckForDelegation(hub, senderAddress, "sender [value transfer]", false);
    senderValueTransferNew =
        senderValueTransfer.deepCopy().decrementBalanceBy(value).dontCheckForDelegation(hub);
    latestAccountSnapshots.put(senderAddress, senderValueTransferNew);

    recipientValueReception =
        deepCopyAndMaybeCheckForDelegation(
            hub, recipientAddress, "recipient [value reception]", true);

    checkState(
        !recipientValueReception.deploymentStatus(),
        "TxInitializationSection: recipient should not have been undergoing deployment before transaction start");

    recipientValueReceptionNew = recipientValueReception.deepCopy().dontCheckForDelegation(hub);

    if (tx.isDeployment()) {
      checkState(
          recipientValueReception.code().isEmpty(),
          "TxInitializationSection: the recipient of a deployment transaction must have empty code");
      checkState(
          recipientValueReception.nonce() == 0,
          "TxInitializationSection: the recipient of a deployment transaction must have zero nonce");

      hub.transients()
          .conflation()
          .deploymentInfo()
          .newDeploymentWithExecutionAt(
              recipientAddress, tx.getBesuTransaction().getInit().orElse(Bytes.EMPTY));

      // this should be useless
      checkState(
          hub.deploymentStatusOf(recipientAddress),
          "at this point the recipient should be undergoing deployment");
      checkState(
          recipientValueReception.deploymentNumber() + 1
              == hub.deploymentNumberOf(recipientAddress),
          "Deployment status should be true and deployment number should have incremented by 1");

      final Bytecode initCode = new Bytecode(tx.getBesuTransaction().getInit().orElse(Bytes.EMPTY));
      recipientValueReceptionNew
          .incrementNonceByOne()
          .incrementBalanceBy(value)
          .code(initCode)
          .turnOnWarmth()
          .setDeploymentInfo(hub);
    } else {
      recipientValueReceptionNew.incrementBalanceBy(value).turnOnWarmth();
    }
    latestAccountSnapshots.put(recipientAddress, recipientValueReceptionNew);
    recipientUndoingValueReception = recipientValueReceptionNew.deepCopy();

    // delegate or recipient
    delegateOrRecipient =
        deepCopyAndMaybeCheckForDelegation(
            hub,
            delegateOrRecipientAddress,
            delegateAddress.isPresent()
                ? "delegate [reading]"
                : "recipient [reading instead of delegate]",
            true);
    delegateOrRecipientNew = delegateOrRecipient.deepCopy().turnOnWarmth();
    latestAccountSnapshots.put(delegateOrRecipientAddress, delegateOrRecipientNew);

    miscFragment = ImcFragment.forTxInit(hub);
    hub.defers().scheduleForContextEntry(miscFragment);

    coinbaseWarmingAccountFragment =
        accountFragmentFactory
            .makeWithTrm(
                coinbase,
                coinbaseNew,
                coinbaseAddress,
                DomSubStampsSubFragment.standardDomSubStamps(getHubStamp(), domSubOffset()),
                TransactionProcessingType.USER)
            .dontCheckForDelegation(hub);

    gasPaymentAccountFragment =
        accountFragmentFactory
            .makeWithTrm(
                senderGasPayment,
                senderGasPaymentNew,
                senderGasPayment.address(),
                DomSubStampsSubFragment.standardDomSubStamps(hubStamp, domSubOffset()),
                TransactionProcessingType.USER)
            .checkForDelegationIfAccountHasCode(hub);

    valueSendingAccountFragment =
        accountFragmentFactory
            .make(
                senderValueTransfer,
                senderValueTransferNew,
                DomSubStampsSubFragment.standardDomSubStamps(hubStamp, domSubOffset()),
                TransactionProcessingType.USER)
            .dontCheckForDelegation(hub);

    valueReceptionAccountFragment =
        accountFragmentFactory
            .makeWithTrm(
                recipientValueReception,
                recipientValueReceptionNew,
                recipientValueReception.address(),
                DomSubStampsSubFragment.standardDomSubStamps(hubStamp, domSubOffset()),
                TransactionProcessingType.USER)
            .checkForDelegationIfAccountHasCode(hub);

    delegateAccountFragment =
        accountFragmentFactory
            .makeWithTrm(
                delegateOrRecipient,
                delegateOrRecipientNew,
                delegateOrRecipient.address(),
                DomSubStampsSubFragment.standardDomSubStamps(hubStamp, domSubOffset()),
                TransactionProcessingType.USER)
            .requiresRomlex(true)
            .checkForDelegationIfAccountHasCode(hub);

    initializationContextFragment = ContextFragment.initializeExecutionContext(hub);

    hub.state.processingPhase(TX_EXEC);
  }

  @Override
  public void resolveAtEndTransaction(
      Hub hub, WorldView state, Transaction tx, boolean isSuccessful) {

    addFragment(hub().txStack().current().userTransactionFragment());
    addFragment(miscFragment);
    addFragment(coinbaseWarmingAccountFragment);
    addFragment(gasPaymentAccountFragment);
    addFragment(valueSendingAccountFragment);
    addFragment(valueReceptionAccountFragment);
    addFragment(delegateAccountFragment);

    if (!isSuccessful) {
      senderUndoingValueTransfer = senderValueTransferNew.deepCopy().setDeploymentNumber(hub);
      senderUndoingValueTransferNew = senderValueTransfer.deepCopy().setDeploymentNumber(hub);

      recipientUndoingValueReception =
          recipientValueReceptionNew.deepCopy().setDeploymentNumber(hub);
      recipientUndoingValueReceptionNew =
          recipientValueReception.deepCopy().setDeploymentNumber(hub).turnOnWarmth();

      if (tx.getTo().isEmpty()) {
        recipientUndoingValueReception
            .deploymentStatus(true)
            .code(recipientValueReceptionNew.code());
      }

      final int revertStamp = hub.currentFrame().revertStamp();

      this.addFragment( // ACC i +  (sender)
          accountFragmentFactory.make(
              senderUndoingValueTransfer,
              senderUndoingValueTransferNew,
              DomSubStampsSubFragment.revertWithCurrentDomSubStamps(
                  hubStamp, revertStamp, domSubOffset()),
              TransactionProcessingType.USER));

      this.addFragment( // ACC i +  (recipient)
          accountFragmentFactory.make(
              recipientUndoingValueReception,
              recipientUndoingValueReceptionNew,
              DomSubStampsSubFragment.revertWithCurrentDomSubStamps(
                  hubStamp, revertStamp, domSubOffset()),
              TransactionProcessingType.USER));
    }

    this.addFragment(initializationContextFragment);
  }

  protected int domSubOffset() {
    return domSubOffset++;
  }

  private AccountSnapshot deepCopyAndMaybeCheckForDelegation(
      Hub hub, Address address, String purpose, boolean checkingForDelegationIsDesirable) {

    if (!latestAccountSnapshots.containsKey(address)) {
      System.out.println(
          String.format(
              "Address %s for purpose \"%s\" not present in account snapshot map",
              address, purpose));
      return canonicalWithoutFrame(hub, world, address);
    }

    final AccountSnapshot snapshot = latestAccountSnapshots.get(address).deepCopy();
    if (checkingForDelegationIsDesirable) {
      snapshot.checkForDelegationIfAccountHasCode(hub);
    }

    return snapshot;
  }
}
