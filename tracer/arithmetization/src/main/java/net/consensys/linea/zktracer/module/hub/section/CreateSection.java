/*
 * Copyright ConsenSys Inc.
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

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.CreateScenarioFragment.CreateScenario.*;
import static net.consensys.linea.zktracer.opcode.OpCode.*;
import static net.consensys.linea.zktracer.types.AddressUtils.getDeploymentAddress;

import java.util.Optional;
import lombok.Getter;
import net.consensys.linea.zktracer.module.hub.AccountSnapshot;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.TransactionProcessingType;
import net.consensys.linea.zktracer.module.hub.defer.*;
import net.consensys.linea.zktracer.module.hub.fragment.ContextFragment;
import net.consensys.linea.zktracer.module.hub.fragment.DomSubStampsSubFragment;
import net.consensys.linea.zktracer.module.hub.fragment.account.AccountFragment;
import net.consensys.linea.zktracer.module.hub.fragment.account.RlpAddrSubFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.ImcFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.MxpCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.StpCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.opcodes.CreateOobCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.opcodes.XCreateOobCall;
import net.consensys.linea.zktracer.module.hub.fragment.scenario.CreateScenarioFragment;
import net.consensys.linea.zktracer.module.hub.fragment.scenario.CreateScenarioFragment.CreateScenario;
import net.consensys.linea.zktracer.module.hub.signals.AbortingConditions;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import net.consensys.linea.zktracer.module.shakiradata.ShakiraDataOperation;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.units.bigints.UInt256;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.evm.account.Account;
import org.hyperledger.besu.evm.account.AccountState;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.internal.Words;
import org.hyperledger.besu.evm.operation.Operation;
import org.hyperledger.besu.evm.worldstate.WorldView;

public final class CreateSection extends TraceSection
    implements PostOpcodeDefer,
        ContextEntryDefer,
        PostRollbackDefer,
        ContextReEntryDefer,
        EndTransactionDefer {

  public static final short NB_ROWS_HUB_CREATE = 11;

  private final Address creatorAddress;
  private final Address createeAddress;

  private final AccountFragment.AccountFragmentFactory accountFragmentFactory;

  // First account rows
  private AccountSnapshot firstCreator;
  private AccountSnapshot firstCreatee;
  private AccountSnapshot firstCreatorNew;
  private AccountSnapshot firstCreateeNew;

  // Second account rows
  private AccountSnapshot secondCreator;
  private AccountSnapshot secondCreatee;
  private AccountSnapshot secondCreatorNew;
  private AccountSnapshot secondCreateeNew;

  // Second account rows
  private AccountSnapshot thirdCreator;
  private AccountSnapshot thirdCreatee;
  private AccountSnapshot thirdCreatorNew;
  private AccountSnapshot thirdCreateeNew;

  private RlpAddrSubFragment rlpAddrSubFragment;

  @Getter public final CreateScenarioFragment scenarioFragment; // row i + 0
  final ContextFragment currentContextFragment; // row i + 1
  final ImcFragment imcFragment; // row i + 2
  private ContextFragment finalContextFragment; // row i+?

  private boolean requiresRomLex;
  private final Wei value;
  private boolean success = false;

  private final int hubStamp;

  public CreateSection(Hub hub, MessageFrame frame) {
    super(hub, maxNumberOfLines(hub.pch().exceptions(), hub.pch().abortingConditions()));
    accountFragmentFactory = hub.factories().accountFragment();
    hubStamp = hub.stamp();

    creatorAddress = frame.getRecipientAddress();
    createeAddress = getDeploymentAddress(frame, hub.opCodeData(frame));
    value = Wei.of(UInt256.fromBytes(frame.getStackItem(0)));

    scenarioFragment = new CreateScenarioFragment();
    currentContextFragment = ContextFragment.readCurrentContextData(hub);
    imcFragment = ImcFragment.empty(hub);

    this.addStack(hub);
    this.addFragment(scenarioFragment);
    this.addFragment(currentContextFragment);
    this.addFragment(imcFragment);

    refineCreateScenario(hub, frame);
    scheduleSection(hub);

    final short exceptions = hub.pch().exceptions();

    // STATICX case
    if (Exceptions.staticFault(exceptions)) {
      return;
    }

    // MAXCSX case: EIP-3860 in Shanghai
    if (Exceptions.maxCodeSizeException(exceptions)) {
      imcFragment.callOob(new XCreateOobCall());
      return;
    }

    // MXPX case
    final MxpCall mxpCall = MxpCall.newMxpCall(hub);
    imcFragment.callMxp(mxpCall);
    checkArgument(
        mxpCall.mxpx == Exceptions.memoryExpansionException(exceptions),
        "CREATE(2): mxp and hub disagree on MXPX");
    if (mxpCall.mxpx) {
      return;
    }

    // OOGX case
    final StpCall stpCall = new StpCall(hub, frame, mxpCall.getGasMxp());
    imcFragment.callStp(stpCall);
    checkArgument(
        stpCall.outOfGasException() == Exceptions.outOfGasException(exceptions),
        "CREATE(2): stp and hub disagree on OOGX");
    if (Exceptions.outOfGasException(exceptions)) {
      return;
    }

    // The CREATE(2) is now unexceptional
    checkArgument(Exceptions.none(exceptions), "CREATE(2): unexpectedly exceptional");
    hub.currentFrame().childSpanningSection(this);

    final CreateOobCall oobCall = (CreateOobCall) imcFragment.callOob(new CreateOobCall());

    firstCreator = AccountSnapshot.canonical(hub, frame.getWorldUpdater(), creatorAddress);
    firstCreatee = AccountSnapshot.canonical(hub, frame.getWorldUpdater(), createeAddress);

    final boolean aborts = scenarioFragment.getScenario() == CREATE_ABORT;
    final boolean failedCreate = scenarioFragment.isFailedCreate();
    final boolean emptyInitCode =
        scenarioFragment.getScenario() == CREATE_EMPTY_INIT_CODE_WONT_REVERT;

    checkArgument(
        oobCall.isAbortingCondition() == aborts, "CREATE(2): oob and hub disagree on ABORT");
    if (aborts) {
      this.traceAbort(hub);
      return;
    }

    // The CREATE(2) is now unexceptional, unaborted
    rlpAddrSubFragment = RlpAddrSubFragment.makeFragment(hub, createeAddress);

    final long offset = Words.clampedToLong(frame.getStackItem(1));
    final long size = Words.clampedToLong(frame.getStackItem(2));

    // Trigger MMU & SHAKIRA to hash the (non-empty) InitCode of CREATE2 - even for failed CREATE2
    if (nontrivialCreate2(hub.opCode(), size)) {
      final Bytes create2InitCode = frame.shadowReadMemory(offset, size);

      final MmuCall mmuCall = MmuCall.create2(hub, create2InitCode, failedCreate);
      imcFragment.callMmu(mmuCall);

      final ShakiraDataOperation shakiraDataOperation =
          new ShakiraDataOperation(hub.stamp(), create2InitCode);
      hub.shakiraData().call(shakiraDataOperation);

      writeHashInfoResult(shakiraDataOperation.result());
    }

    if (failedCreate) {
      finalContextFragmentSquashesReturnData(hub);
      commonValues.payGasPaidOutOfPocket(hub);
      return;
    }

    if (emptyInitCode) {
      success = true;
      finalContextFragmentSquashesReturnData(hub);
      hub.transients().conflation().deploymentInfo().newDeploymentSansExecutionAt(createeAddress);
      return;
    }

    // unexceptional, unaborted, non-failing, non-emptyInitCode CREATE(2)
    // we charge for the gas paid out of pocket
    commonValues.payGasPaidOutOfPocket(hub);

    requiresRomLex = true;
    hub.romLex().callRomLex(frame);
    hub.transients()
        .conflation()
        .deploymentInfo()
        .newDeploymentWithExecutionAt(createeAddress, frame.shadowReadMemory(offset, size));

    // Note: the case CREATE2 has been set before, we need to do it even in the failure case
    if (hub.opCode() == CREATE) {
      final MmuCall mmuCall = MmuCall.create(hub);
      imcFragment.callMmu(mmuCall);
    }

    finalContextFragment = ContextFragment.initializeExecutionContext(hub);
  }

  @Override
  public void resolveUponContextEntry(Hub hub, MessageFrame frame) {
    firstCreatorNew =
        AccountSnapshot.canonical(hub, frame.getWorldUpdater(), firstCreator.address())
            // .raiseNonceByOne() // the nonce was already raised
            .decrementBalanceBy(value);
    firstCreateeNew =
        AccountSnapshot.canonical(hub, frame.getWorldUpdater(), firstCreatee.address())
            .incrementNonceByOne()
            .incrementBalanceBy(value);

    final AccountFragment creatorAccountFragment =
        accountFragmentFactory.make(
            firstCreator,
            firstCreatorNew,
            DomSubStampsSubFragment.standardDomSubStamps(this.hubStamp(), 0),
            TransactionProcessingType.USER);
    creatorAccountFragment.rlpAddrSubFragment(rlpAddrSubFragment);

    final AccountFragment createeAccountFragment =
        accountFragmentFactory.makeWithTrm(
            firstCreatee,
            firstCreateeNew,
            createeAddress.trimLeadingZeros(),
            DomSubStampsSubFragment.standardDomSubStamps(this.hubStamp(), 1),
            TransactionProcessingType.USER);

    createeAccountFragment.requiresRomlex(requiresRomLex);

    this.addFragments(creatorAccountFragment, createeAccountFragment);
  }

  @Override
  public void resolveAtContextReEntry(Hub hub, CallFrame frame) {
    success = !frame.frame().getStackItem(0).isZero();

    CreateScenario scenario = scenarioFragment.getScenario();

    switch (scenario) {
      case CREATE_FAILURE_CONDITION_WONT_REVERT, CREATE_EMPTY_INIT_CODE_WONT_REVERT -> {
        if (scenario == CREATE_FAILURE_CONDITION_WONT_REVERT)
          checkState(
              !success,
              "CreateSection: %s scenario requires CREATE failure, yet success = %s",
              CREATE_FAILURE_CONDITION_WONT_REVERT,
              success);
        if (scenario == CREATE_EMPTY_INIT_CODE_WONT_REVERT)
          checkState(
              success,
              "CreateSection: %s scenario requires CREATE success, yet success = %s",
              CREATE_EMPTY_INIT_CODE_WONT_REVERT,
              success);

        firstCreatorNew =
            AccountSnapshot.canonical(hub, frame.frame().getWorldUpdater(), creatorAddress);
        firstCreateeNew =
            AccountSnapshot.canonical(hub, frame.frame().getWorldUpdater(), createeAddress);

        final AccountFragment firstCreatorFragment =
            accountFragmentFactory.make(
                firstCreator,
                firstCreatorNew,
                DomSubStampsSubFragment.standardDomSubStamps(this.hubStamp(), 0),
                TransactionProcessingType.USER);
        firstCreatorFragment.rlpAddrSubFragment(rlpAddrSubFragment);

        final AccountFragment firstCreateeFragment =
            accountFragmentFactory.makeWithTrm(
                firstCreatee,
                firstCreateeNew,
                createeAddress.trimLeadingZeros(),
                DomSubStampsSubFragment.standardDomSubStamps(this.hubStamp(), 1),
                TransactionProcessingType.USER);

        this.addFragments(firstCreatorFragment, firstCreateeFragment);
      }
      case CREATE_NON_EMPTY_INIT_CODE_SUCCESS_WONT_REVERT -> {
        if (success) return;

        scenarioFragment.setScenario(CREATE_NON_EMPTY_INIT_CODE_FAILURE_WONT_REVERT);

        secondCreator = firstCreatorNew.deepCopy().setDeploymentNumber(hub);
        secondCreatorNew = firstCreator.deepCopy().setDeploymentNumber(hub).incrementNonceByOne();

        secondCreatee = firstCreateeNew.deepCopy().setDeploymentNumber(hub);
        secondCreateeNew = firstCreatee.deepCopy().setDeploymentNumber(hub).turnOnWarmth();

        final int childRevertStamp = hub.getLastChildCallFrame(frame).revertStamp();

        final AccountFragment undoCreatorAfterFailedDeployment =
            accountFragmentFactory.make(
                secondCreator,
                secondCreatorNew,
                DomSubStampsSubFragment.revertsWithChildDomSubStamps(
                    this.hubStamp(), childRevertStamp, 0),
                TransactionProcessingType.USER);

        final AccountFragment undoCreateeAfterFailedDeployment =
            accountFragmentFactory.make(
                secondCreatee,
                secondCreateeNew,
                DomSubStampsSubFragment.revertsWithChildDomSubStamps(
                    this.hubStamp(), childRevertStamp, 1),
                TransactionProcessingType.USER);

        this.addFragments(undoCreatorAfterFailedDeployment, undoCreateeAfterFailedDeployment);
      }
      default ->
          throw new IllegalStateException(
              scenario.name() + " not allowed when resolving at context re-entry");
    }
  }

  @Override
  public void resolveUponRollback(Hub hub, MessageFrame messageFrame, CallFrame callFrame) {

    final CreateScenario scenario = scenarioFragment.getScenario();
    checkState(
        scenarioFragment
            .getScenario()
            .isAnyOf(
                CREATE_FAILURE_CONDITION_WONT_REVERT,
                CREATE_EMPTY_INIT_CODE_WONT_REVERT,
                CREATE_NON_EMPTY_INIT_CODE_SUCCESS_WONT_REVERT,
                CREATE_NON_EMPTY_INIT_CODE_FAILURE_WONT_REVERT),
        "CreateSection: %s CREATE-scenario not allowed when resolving upon rollback",
        scenarioFragment.getScenario());

    final int revertStamp = callFrame.revertStamp();

    if (scenario.isAnyOf(
        CREATE_FAILURE_CONDITION_WONT_REVERT,
        CREATE_EMPTY_INIT_CODE_WONT_REVERT,
        CREATE_NON_EMPTY_INIT_CODE_SUCCESS_WONT_REVERT)) {
      oneTakeRevert(hub, revertStamp);
    }

    if (scenario == CREATE_NON_EMPTY_INIT_CODE_FAILURE_WONT_REVERT) {
      revertAfterFailedDeployment(hub, revertStamp);
    }

    scenarioFragment.setScenario(switchToRevertingScenario(scenario));
  }

  private void oneTakeRevert(Hub hub, int revertStamp) {

    secondCreator = firstCreatorNew.deepCopy().setDeploymentNumber(hub);
    secondCreatorNew = firstCreator.deepCopy().setDeploymentNumber(hub);

    secondCreatee = firstCreateeNew.deepCopy().setDeploymentNumber(hub);
    secondCreateeNew = firstCreatee.deepCopy().setDeploymentNumber(hub);

    final AccountFragment undoCreator =
        accountFragmentFactory.make(
            secondCreator,
            secondCreatorNew,
            DomSubStampsSubFragment.revertWithCurrentDomSubStamps(hubStamp, revertStamp, 0),
            TransactionProcessingType.USER);

    final AccountFragment undoCreatee =
        accountFragmentFactory.make(
            secondCreatee,
            secondCreateeNew,
            DomSubStampsSubFragment.revertWithCurrentDomSubStamps(hubStamp, revertStamp, 1),
            TransactionProcessingType.USER);

    this.addFragments(undoCreator, undoCreatee);
  }

  private void revertAfterFailedDeployment(Hub hub, int revertStamp) {

    thirdCreator = secondCreatorNew.deepCopy().setDeploymentNumber(hub);
    thirdCreatorNew = firstCreator.deepCopy().setDeploymentNumber(hub);

    thirdCreatee = secondCreateeNew.deepCopy().setDeploymentNumber(hub);
    thirdCreateeNew = firstCreatee.deepCopy().setDeploymentNumber(hub);

    final AccountFragment undoCreatorFinal =
        accountFragmentFactory.make(
            thirdCreator,
            thirdCreatorNew,
            DomSubStampsSubFragment.revertWithCurrentDomSubStamps(hubStamp, revertStamp, 2),
            TransactionProcessingType.USER);

    final AccountFragment undoCreateeFinal =
        accountFragmentFactory.make(
            thirdCreatee,
            thirdCreateeNew,
            DomSubStampsSubFragment.revertWithCurrentDomSubStamps(hubStamp, revertStamp, 3),
            TransactionProcessingType.USER);

    this.addFragments(undoCreatorFinal, undoCreateeFinal);
  }

  private static short maxNumberOfLines(final short exceptions, final AbortingConditions abort) {
    if (Exceptions.any(exceptions)) {
      return 6;
    }
    if (abort.any()) {
      return 7;
    }
    return NB_ROWS_HUB_CREATE; // Note: could be lower for unreverted successful CREATE(s)
  }

  private void traceAbort(final Hub hub) {
    final AccountFragment creatorAccountFragment =
        accountFragmentFactory.make(
            firstCreator,
            firstCreator,
            DomSubStampsSubFragment.standardDomSubStamps(this.hubStamp(), 0),
            TransactionProcessingType.USER);

    finalContextFragmentSquashesReturnData(hub);

    this.addFragments(creatorAccountFragment, finalContextFragment);
  }

  private void finalContextFragmentSquashesReturnData(Hub hub) {
    finalContextFragment = ContextFragment.nonExecutionProvidesEmptyReturnData(hub);
  }

  private boolean hasEmptyInitCode(Hub hub) {
    return hub.transients().op().initCodeSegment().isEmpty();
  }

  private void refineCreateScenario(Hub hub, MessageFrame frame) {
    if (hub.isExceptional()) {
      scenarioFragment.setScenario(CREATE_EXCEPTION);
      return;
    }

    if (hub.pch().abortingConditions().any()) {
      scenarioFragment.setScenario(CREATE_ABORT);
      return;
    }

    if (raisesFailureCondition(frame)) {
      scenarioFragment.setScenario(CREATE_FAILURE_CONDITION_WONT_REVERT);
      return;
    }

    scenarioFragment.setScenario(
        hasEmptyInitCode(hub)
            ? CREATE_EMPTY_INIT_CODE_WONT_REVERT
            : CREATE_NON_EMPTY_INIT_CODE_SUCCESS_WONT_REVERT);
  }

  private void scheduleSection(Hub hub) {
    final CreateScenario scenario = scenarioFragment.getScenario();
    final CallFrame currentFrame = hub.currentFrame();
    switch (scenario) {
      case CREATE_EXCEPTION -> {}
      case CREATE_ABORT -> hub.defers().scheduleForPostExecution(this); // unlatch the stack
      case CREATE_FAILURE_CONDITION_WONT_REVERT, CREATE_EMPTY_INIT_CODE_WONT_REVERT -> {
        hub.defers().scheduleForContextReEntry(this, currentFrame);
        hub.defers().scheduleForPostRollback(this, currentFrame);
        hub.defers().scheduleForEndTransaction(this);
      }
      case CREATE_NON_EMPTY_INIT_CODE_SUCCESS_WONT_REVERT -> {
        // The current execution context pays (63/64)ths of it current gas to the child context
        // To get the success bit of the CREATE(2) operation
        hub.defers().scheduleForContextEntry(this);
        hub.defers().scheduleForContextReEntry(this, currentFrame);
        hub.defers().scheduleForPostRollback(this, currentFrame);
        hub.defers().scheduleForEndTransaction(this);

        // we capture revert information about the child context: CCSR and CCRS
        hub.defers().scheduleForContextReEntry(imcFragment, hub.currentFrame());
      }
      default ->
          throw new IllegalStateException(
              scenario.name() + " not allowed when defining the schedule");
    }
  }

  private boolean raisesFailureCondition(MessageFrame frame) {

    final Optional<Account> deploymentAccount =
        Optional.ofNullable(frame.getWorldUpdater().get(createeAddress));
    final boolean createdAddressHasNonZeroNonce =
        deploymentAccount.map(a -> a.getNonce() != 0).orElse(false);
    final boolean createdAddressHasNonEmptyCode =
        deploymentAccount.map(AccountState::hasCode).orElse(false);

    return createdAddressHasNonZeroNonce || createdAddressHasNonEmptyCode;
  }

  // we unlatched the stack after a CREATE if and only if we don't "contextEnter" the CREATE.
  // "failure condition CREATE's" do enter the CREATE context.
  @Override
  public void resolvePostExecution(
      Hub hub, MessageFrame frame, Operation.OperationResult operationResult) {
    checkState(
        scenarioFragment.isAbortedCreate(),
        "CreateSection: we resolve a CREATE(2) post execution only if it's an aborted CREATE(2), yet scenario = %s",
        scenarioFragment.getScenario());
    hub.unlatchStack(frame, this);
  }

  @Override
  public void resolveAtEndTransaction(
      Hub hub, WorldView state, Transaction tx, boolean isSuccessful) {
    addFragment(finalContextFragment);
  }

  private boolean nontrivialCreate2(OpCode opCode, long size) {
    return (opCode == CREATE2 && size != 0);
  }
}
