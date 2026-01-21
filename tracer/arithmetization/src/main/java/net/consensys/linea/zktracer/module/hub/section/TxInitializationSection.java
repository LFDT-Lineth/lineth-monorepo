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
import static net.consensys.linea.zktracer.module.hub.AccountSnapshot.canonical;
import static net.consensys.linea.zktracer.module.hub.HubProcessingPhase.TX_EXEC;

import lombok.Getter;
import net.consensys.linea.zktracer.module.hub.AccountSnapshot;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.TransactionProcessingType;
import net.consensys.linea.zktracer.module.hub.defer.EndTransactionDefer;
import net.consensys.linea.zktracer.module.hub.fragment.ContextFragment;
import net.consensys.linea.zktracer.module.hub.fragment.DomSubStampsSubFragment;
import net.consensys.linea.zktracer.module.hub.fragment.account.AccountFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.ImcFragment;
import net.consensys.linea.zktracer.module.hub.transients.DeploymentInfo;
import net.consensys.linea.zktracer.types.Bytecode;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.evm.account.Account;
import org.hyperledger.besu.evm.worldstate.WorldView;

public final class TxInitializationSection extends TraceSection implements EndTransactionDefer {

  public static final short NB_ROWS_HUB_INIT = 9;

  @Getter private final int hubStamp;
  final AccountFragment.AccountFragmentFactory accountFragmentFactory;

  private final ImcFragment miscFragment;

  public final AccountFragment coinbaseWarmingAccountFragment;

  private final AccountFragment gasPaymentAccountFragment;
  @Getter private final AccountSnapshot senderGasPayment;
  @Getter private final AccountSnapshot senderGasPaymentNew;

  private final AccountFragment valueSendingAccountFragment;
  @Getter private final AccountSnapshot senderValueTransfer;
  @Getter private final AccountSnapshot senderValueTransferNew;

  private final AccountFragment valueReceptionAccountFragment;
  @Getter private final AccountSnapshot recipientValueReception;
  @Getter private final AccountSnapshot recipientValueReceptionNew;

  @Getter private AccountSnapshot senderUndoingValueTransfer;
  @Getter private AccountSnapshot senderUndoingValueTransferNew;

  @Getter private AccountSnapshot recipientUndoingValueReception;
  @Getter private AccountSnapshot recipientUndoingValueReceptionNew;

  @Getter private final ContextFragment initializationContextFragment;

  /** This is used to generate the Dom / Sub offset */
  private int domSubOffset = 0;

  public TxInitializationSection(Hub hub, WorldView world) {
    super(hub, NB_ROWS_HUB_INIT);
    hub.defers().scheduleForEndTransaction(this);

    hubStamp = hub.stamp();
    accountFragmentFactory = hub.factories().accountFragment();

    hub.txStack().setInitializationSection(this);

    final TransactionProcessingMetadata tx = hub.txStack().current();
    final Address senderAddress = tx.getSender();
    final Address recipientAddress = tx.getEffectiveRecipient();
    final Account senderAccount = world.get(senderAddress);
    final DeploymentInfo deploymentInfo = hub.transients().conflation().deploymentInfo();

    final boolean isDeployment = tx.isDeployment();
    final Wei transactionGasPrice = Wei.of(tx.getEffectiveGasPrice());
    final Wei gasCost = transactionGasPrice.multiply(tx.getBesuTransaction().getGasLimit());

    coinbaseWarmingAccountFragment = makeCoinbaseWarmingFragment(hub, world, tx);

    senderGasPayment =
        canonical(hub, world, senderAccount.getAddress(), senderWarmthAtGasPayment(tx));

    senderGasPaymentNew =
        senderGasPayment.deepCopy().decrementBalanceBy(gasCost).turnOnWarmth().raiseNonceByOne();

    final Wei value = (Wei) tx.getBesuTransaction().getValue();

    senderValueTransfer = senderGasPaymentNew.deepCopy();
    senderValueTransferNew = senderValueTransfer.deepCopy().decrementBalanceBy(value);

    final Account recipientAccount = world.get(recipientAddress);

    if (recipientAccount != null) {
      recipientValueReception =
          tx.senderIsRecipient()
              ? senderValueTransferNew
              : canonical(hub, world, recipientAddress, recipientWarmthAtValueReception(tx));
    } else {
      recipientValueReception =
          AccountSnapshot.fromAddress(
              recipientAddress,
              recipientWarmthAtValueReception(tx),
              deploymentInfo.deploymentNumber(recipientAddress),
              deploymentInfo.getDeploymentStatus(recipientAddress),
              hub.delegationNumberOf(recipientAddress));
    }

    checkState(
        !recipientValueReception.deploymentStatus(),
        "TxInitializationSection: recipient should not have been undergoing deployment before transaction start");

    recipientValueReceptionNew = recipientValueReception.deepCopy();

    if (isDeployment) {
      if (recipientAccount != null) {
        checkState(
            recipientAccount.getCode().equals(Bytes.EMPTY),
            "TxInitializationSection: the recipient of a deployment transaction must have empty code");
        checkState(
            recipientAccount.getNonce() == 0,
            "TxInitializationSection: the recipient of a deployment transaction must have zero nonce");
      }

      deploymentInfo.newDeploymentWithExecutionAt(
          recipientAddress, tx.getBesuTransaction().getInit().orElse(Bytes.EMPTY));

      // this should be useless
      checkState(
          deploymentInfo.getDeploymentStatus(recipientAddress),
          "at this point the recipient should be undergoing deployment");
      checkState(
          recipientValueReception.deploymentNumber() + 1
              == deploymentInfo.deploymentNumber(recipientAddress),
          "Deployment status should be true and deployment number should have incremented by 1");

      final Bytecode initCode = new Bytecode(tx.getBesuTransaction().getInit().orElse(Bytes.EMPTY));
      recipientValueReceptionNew
          .raiseNonceByOne()
          .incrementBalanceBy(value)
          .code(initCode)
          .turnOnWarmth()
          .setDeploymentInfo(deploymentInfo);
    } else {
      recipientValueReceptionNew.incrementBalanceBy(value).turnOnWarmth();
    }
    recipientUndoingValueReception = recipientValueReceptionNew.deepCopy();

    miscFragment = ImcFragment.forTxInit(hub);
    hub.defers().scheduleForContextEntry(miscFragment);

    gasPaymentAccountFragment =
        accountFragmentFactory.makeWithTrm(
            senderGasPayment,
            senderGasPaymentNew,
            senderGasPayment.address(),
            DomSubStampsSubFragment.standardDomSubStamps(hubStamp, domSubOffset()),
            TransactionProcessingType.USER);
    valueSendingAccountFragment =
        accountFragmentFactory.make(
            senderValueTransfer,
            senderValueTransferNew,
            DomSubStampsSubFragment.standardDomSubStamps(hubStamp, domSubOffset()),
            TransactionProcessingType.USER);
    valueReceptionAccountFragment =
        accountFragmentFactory
            .makeWithTrm(
                recipientValueReception,
                recipientValueReceptionNew,
                recipientValueReception.address(),
                DomSubStampsSubFragment.standardDomSubStamps(hubStamp, domSubOffset()),
                TransactionProcessingType.USER)
            .requiresRomlex(true);

    initializationContextFragment = ContextFragment.initializeExecutionContext(hub);

    hub.state.processingPhase(TX_EXEC);
  }

  @Override
  public void resolveAtEndTransaction(
      Hub hub, WorldView state, Transaction tx, boolean isSuccessful) {

    addFragment(hub().txStack().current().userTransactionFragment());
    addFragment(miscFragment);
    addFragment(coinbaseWarmingAccountFragment);
    addFragment(gasPaymentAccountFragment); // ACC i +  (sender: gas payment)
    addFragment(valueSendingAccountFragment); // ACC i +  (sender: value transfer)
    addFragment(valueReceptionAccountFragment); // ACC i +  (recipient: value reception)

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

    this.addFragment(initializationContextFragment); // CON i +
  }

  protected int domSubOffset() {
    return domSubOffset++;
  }

  private AccountFragment makeCoinbaseWarmingFragment(
      final Hub hub, final WorldView world, final TransactionProcessingMetadata tx) {
    final AccountSnapshot coinbase =
        canonical(hub, world, hub.coinbaseAddress(), tx.isCoinbasePreWarmed());
    return accountFragmentFactory.makeWithTrm(
        coinbase,
        coinbase.deepCopy().turnOnWarmth(),
        coinbase.address(),
        DomSubStampsSubFragment.standardDomSubStamps(getHubStamp(), domSubOffset()),
        TransactionProcessingType.USER);
  }

  private boolean senderWarmthAtGasPayment(final TransactionProcessingMetadata tx) {
    return tx.isSenderPreWarmed() || tx.senderIsCoinbase();
  }

  private boolean recipientWarmthAtValueReception(TransactionProcessingMetadata tx) {
    return tx.isRecipientPreWarmed() || tx.recipientIsCoinbase();
  }
}
