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
package net.consensys.linea.zktracer.module.hub.section.halt;

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.SelfdestructScenarioFragment.SelfdestructScenario.*;
import static net.consensys.linea.zktracer.module.hub.signals.Exceptions.OUT_OF_GAS_EXCEPTION;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import lombok.Getter;
import net.consensys.linea.zktracer.module.hub.AccountSnapshot;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.TransactionProcessingType;
import net.consensys.linea.zktracer.module.hub.defer.AfterTransactionFinalizationDefer;
import net.consensys.linea.zktracer.module.hub.defer.EndTransactionDefer;
import net.consensys.linea.zktracer.module.hub.defer.PostOpcodeDefer;
import net.consensys.linea.zktracer.module.hub.defer.PostRollbackDefer;
import net.consensys.linea.zktracer.module.hub.fragment.ContextFragment;
import net.consensys.linea.zktracer.module.hub.fragment.DomSubStampsSubFragment;
import net.consensys.linea.zktracer.module.hub.fragment.account.AccountFragment;
import net.consensys.linea.zktracer.module.hub.fragment.scenario.SelfdestructScenarioFragment;
import net.consensys.linea.zktracer.module.hub.section.TraceSection;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import net.consensys.linea.zktracer.types.Bytecode;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.operation.Operation;
import org.hyperledger.besu.evm.worldstate.WorldView;

public final class SelfdestructSection extends TraceSection
    implements PostOpcodeDefer,
        PostRollbackDefer,
        EndTransactionDefer,
        AfterTransactionFinalizationDefer {

  public static final short NB_ROWS_HUB_SELFDESTRUCT = 8; // up to 8 = 1 + 7 rows

  final int id;
  final int hubStamp;
  final TransactionProcessingMetadata transactionProcessingMetadata;
  final short exceptions;

  final SelfdestructScenarioFragment selfdestructScenarioFragment;

  final Bytes recipientAddressUntrimmed;
  final Address recipientAddress;

  final AccountSnapshot selfdestructor;
  AccountSnapshot selfdestructorNew;
  AccountSnapshot recipient;
  AccountSnapshot recipientNew;
  AccountSnapshot accountWipingNew;

  @Getter boolean selfDestructWasReverted = false;

  ContextFragment finalUnexceptionalContextFragment;

  public SelfdestructSection(Hub hub, MessageFrame frame) {
    super(hub, NB_ROWS_HUB_SELFDESTRUCT);

    // Init
    id = hub.currentFrame().id();
    transactionProcessingMetadata = hub.txStack().current();
    hubStamp = hub.stamp();
    exceptions = hub.pch().exceptions();

    // Account
    final Address addressWhichMaySelfDestruct = frame.getRecipientAddress();
    selfdestructor = AccountSnapshot.canonical(hub, addressWhichMaySelfDestruct);

    // Recipient
    recipientAddressUntrimmed = frame.getStackItem(0);
    recipientAddress = Address.extract(Bytes32.leftPad(recipientAddressUntrimmed));

    // SCN fragment
    selfdestructScenarioFragment = new SelfdestructScenarioFragment();
    if (Exceptions.any(exceptions)) {
      selfdestructScenarioFragment.setScenario(SELFDESTRUCT_EXCEPTION);
    }

    // CON fragment (1)
    final ContextFragment readCurrentContext = ContextFragment.readCurrentContextData(hub);

    this.addStack(hub); // stack fragments
    this.addFragment(selfdestructScenarioFragment); // scenario fragment
    this.addFragment(readCurrentContext);

    // STATICX case
    if (Exceptions.staticFault(exceptions)) {
      return;
    }

    // OOGX case
    if (Exceptions.any(exceptions)) {
      checkArgument(
          exceptions == OUT_OF_GAS_EXCEPTION,
          "SELFDESTRUCT: lowest priority exception should be %s",
          OUT_OF_GAS_EXCEPTION);

      recipient =
          selfdestructTargetsItself()
              ? selfdestructor
              : AccountSnapshot.canonical(hub, frame.getWorldUpdater(), recipientAddress);

      final AccountFragment selfdestructorFirstAccountFragment =
          hub.factories()
              .accountFragment()
              .make(
                  selfdestructor,
                  selfdestructor,
                  DomSubStampsSubFragment.standardDomSubStamps(this.hubStamp(), 0),
                  TransactionProcessingType.USER);

      final AccountFragment recipientFirstAccountFragment =
          hub.factories()
              .accountFragment()
              .makeWithTrm(
                  recipient,
                  recipient,
                  recipientAddressUntrimmed,
                  DomSubStampsSubFragment.standardDomSubStamps(this.hubStamp(), 1),
                  TransactionProcessingType.USER);

      this.addFragment(selfdestructorFirstAccountFragment);
      this.addFragment(recipientFirstAccountFragment);
      return;
    }

    // Unexceptional case
    finalUnexceptionalContextFragment = ContextFragment.executionProvidesEmptyReturnData(hub);

    final Map<EphemeralAccount, List<AttemptedSelfDestruct>> unexceptionalSelfDestructMap =
        hub.txStack().current().getUnexceptionalSelfDestructMap();

    final EphemeralAccount ephemeralAccount =
        new EphemeralAccount(selfdestructor.address(), selfdestructor.deploymentNumber());

    if (unexceptionalSelfDestructMap.containsKey(ephemeralAccount)) {
      unexceptionalSelfDestructMap
          .get(ephemeralAccount)
          .add(new AttemptedSelfDestruct(hubStamp, hub.currentFrame()));
    } else {
      unexceptionalSelfDestructMap.put(
          new EphemeralAccount(addressWhichMaySelfDestruct, selfdestructor.deploymentNumber()),
          new ArrayList<>(List.of(new AttemptedSelfDestruct(hubStamp, hub.currentFrame()))));
    }

    hub.defers().scheduleForPostExecution(this);
    hub.defers().scheduleForPostRollback(this, hub.currentFrame());
    hub.defers().scheduleForEndTransaction(this);

    if (!selfdestructTargetsItself()) {
      recipient = AccountSnapshot.canonical(hub, frame.getWorldUpdater(), recipientAddress);
    }
  }

  private boolean accountFragmentWiping() {
    // In Cancun, the account fragment is wiped only if it didn't have code initially
    return !transactionProcessingMetadata
        .hadCodeInitiallyMap()
        .get(selfdestructor.address())
        .hadCode();
  }

  private boolean softAccountWiping() {
    return transactionProcessingMetadata
        .hadCodeInitiallyMap()
        .get(selfdestructor.address())
        .hadCode();
  }

  @Override
  public void resolvePostExecution(
      Hub hub, MessageFrame frame, Operation.OperationResult operationResult) {

    final boolean isDeployment = frame.getType() == MessageFrame.Type.CONTRACT_CREATION;
    checkState(
        isDeployment == selfdestructor.deploymentStatus(),
        "SELFDESTRUCT inconsistency: frame deployment = %s but selfdestructor's deployment status = %s",
        isDeployment,
        selfdestructor.deploymentStatus());

    selfdestructorNew = AccountSnapshot.canonical(hub, selfdestructor.address());
    if (isDeployment) {
      selfdestructorNew = selfdestructorNew.deploymentStatus(false);
      selfdestructorNew.code(Bytecode.EMPTY);
    }

    if (!selfdestructorNew.balance().isZero()) {

      // sanity checks
      checkState(
          selfdestructTargetsItself(),
          "If post SELFDESTRUCT the seldestructor's balance is nonzero then SELFDESTRUCT targets self");
      checkState(
          softAccountWiping(),
          "If post SELFDESTRUCT the seldestructor's balance is nonzero then it is a soft account wipe");
      checkState(
          selfdestructorNew.balance().equals(selfdestructor.balance()),
          "If post SELFDESTRUCT the seldestructor's balance is nonzero then the balance should not have changed");

      selfdestructorNew.setBalanceToZero();
    }

    if (selfdestructTargetsItself()) {
      recipient = selfdestructorNew.deepCopy();
    }
    recipientNew =
        AccountSnapshot.canonical(hub, recipientAddress)
            .setDeploymentStatus(recipient.deploymentStatus())
            .code(recipient.code());

    final AccountFragment selfdestructorFirstAccountFragment =
        hub.factories()
            .accountFragment()
            .make(
                selfdestructor,
                selfdestructorNew,
                DomSubStampsSubFragment.standardDomSubStamps(hubStamp, 0),
                TransactionProcessingType.USER);
    final AccountFragment recipientFirstAccountFragment =
        hub.factories()
            .accountFragment()
            .makeWithTrm(
                recipient,
                recipientNew,
                recipientAddressUntrimmed,
                DomSubStampsSubFragment.standardDomSubStamps(hubStamp, 1),
                TransactionProcessingType.USER);

    this.addFragment(selfdestructorFirstAccountFragment);
    this.addFragment(recipientFirstAccountFragment);
  }

  @Override
  public void resolveUponRollback(Hub hub, MessageFrame messageFrame, CallFrame callFrame) {
    // Undo the modifications we applied to selfdestructorFirstAccountFragment and
    // recipientFirstAccountFragment
    final AccountFragment selfDestroyerUndoingAccountFragment =
        hub.factories()
            .accountFragment()
            .make(
                selfdestructorNew.deepCopy().setDeploymentNumber(hub),
                selfdestructor.deepCopy().setDeploymentNumber(hub),
                DomSubStampsSubFragment.revertWithCurrentDomSubStamps(
                    hubStamp, callFrame.revertStamp(), 2),
                TransactionProcessingType.USER);

    final AccountFragment recipientUndoingAccountFragment =
        hub.factories()
            .accountFragment()
            .make(
                recipientNew.deepCopy().setDeploymentNumber(hub),
                recipient.deepCopy().setDeploymentNumber(hub),
                DomSubStampsSubFragment.revertWithCurrentDomSubStamps(
                    hubStamp, callFrame.revertStamp(), 3),
                TransactionProcessingType.USER);

    this.addFragment(selfDestroyerUndoingAccountFragment);
    this.addFragment(recipientUndoingAccountFragment);

    selfDestructWasReverted = true;

    selfdestructScenarioFragment.setScenario(SELFDESTRUCT_WILL_REVERT);
  }

  @Override
  public void resolveAtEndTransaction(
      Hub hub, WorldView state, Transaction tx, boolean isSuccessful) {

    if (selfDestructWasReverted) {
      this.addFragment(finalUnexceptionalContextFragment);
      return;
    }

    // beyond this point the selfdestruct was not reverted
    final Map<EphemeralAccount, Integer> effectiveSelfDestructMap =
        transactionProcessingMetadata.getEffectiveSelfDestructMap();
    final EphemeralAccount ephemeralAccount =
        new EphemeralAccount(selfdestructor.address(), selfdestructorNew.deploymentNumber());

    checkArgument(
        effectiveSelfDestructMap.containsKey(ephemeralAccount),
        "If SELFDESTRUCT was not reverted, the effectiveSelfDestructMap should contain the selfdestructor");

    if (accountFragmentWiping()) {
      // This grabs the accounts right after the coinbase and sender got their gas money back
      // in particular this will get the coinbase address post gas reward.
      final AccountSnapshot accountWiping =
          transactionProcessingMetadata.getDestructedAccountsSnapshot().stream()
              .filter(accountSnapshot -> accountSnapshot.address().equals(selfdestructor.address()))
              .findFirst()
              .orElseThrow(() -> new IllegalStateException("Account not found"));

      // We modify the account fragment to reflect the self-destruct time
      final int hubStampOfTheActionableSelfDestruct =
          effectiveSelfDestructMap.get(ephemeralAccount);
      checkArgument(
          hubStamp >= hubStampOfTheActionableSelfDestruct,
          "The hub stamp of any SELFDESTRUCT in need of resolving at transaction end should be >= the stamp of the actionable SELFDESTRUCT");

      if (hubStamp == hubStampOfTheActionableSelfDestruct) {
        selfdestructScenarioFragment.setScenario(SELFDESTRUCT_WONT_REVERT_NOT_YET_MARKED);

        accountWipingNew = accountWiping.deepCopy();
        // the hub's defers.resolvePostTransaction() gets called after the
        // hub's completeLineaTransaction which in turn calls
        // freshDeploymentNumberFinishingSelfdestruct()
        // which raises the deployment number and sets the deployment status to false
        final AccountFragment accountWipingFragment =
            hub.factories()
                .accountFragment()
                .make(
                    accountWiping,
                    accountWipingNew,
                    DomSubStampsSubFragment.selfdestructDomSubStamps(hub, hubStamp),
                    TransactionProcessingType.USER);

        this.addFragment(accountWipingFragment);
        this.addFragment(finalUnexceptionalContextFragment);

        hub.defers().scheduleForAfterTransactionFinalization(this);
      } else {
        selfdestructScenarioFragment.setScenario(SELFDESTRUCT_WONT_REVERT_ALREADY_MARKED);
        this.addFragment(finalUnexceptionalContextFragment);
      }
    } else {
      selfdestructScenarioFragment.setScenario(SELFDESTRUCT_WONT_REVERT_ALREADY_MARKED);
      this.addFragment(finalUnexceptionalContextFragment);
    }
  }

  @Override
  public void resolveAfterTransactionFinalization(Hub hub, WorldView state) {

    hub.transients()
        .conflation()
        .deploymentInfo()
        .deploymentUpdateForSuccessfulSelfDestruct(selfdestructor.address());

    accountWipingNew.wipe(hub.transients().conflation().deploymentInfo());
  }

  private boolean selfdestructTargetsItself() {
    return selfdestructor.address().equals(recipientAddress);
  }
}
