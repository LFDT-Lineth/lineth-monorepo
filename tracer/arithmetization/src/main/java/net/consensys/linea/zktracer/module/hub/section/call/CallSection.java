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

package net.consensys.linea.zktracer.module.hub.section.call;

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.zktracer.module.hub.AccountSnapshot.canonical;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.CallScenarioFragment.CallScenario.*;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.addressToPrecompileFlag;
import static net.consensys.linea.zktracer.opcode.OpCode.CALL;
import static net.consensys.linea.zktracer.types.AddressUtils.*;
import static net.consensys.linea.zktracer.types.Conversions.bytesToBoolean;

import java.util.Optional;
import lombok.Getter;
import lombok.Setter;
import net.consensys.linea.zktracer.module.hub.AccountSnapshot;
import net.consensys.linea.zktracer.module.hub.Factories;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.TransactionProcessingType;
import net.consensys.linea.zktracer.module.hub.defer.ContextEntryDefer;
import net.consensys.linea.zktracer.module.hub.defer.ContextExitDefer;
import net.consensys.linea.zktracer.module.hub.defer.ContextReEntryDefer;
import net.consensys.linea.zktracer.module.hub.defer.EndTransactionDefer;
import net.consensys.linea.zktracer.module.hub.defer.PostOpcodeDefer;
import net.consensys.linea.zktracer.module.hub.defer.PostRollbackDefer;
import net.consensys.linea.zktracer.module.hub.fragment.ContextFragment;
import net.consensys.linea.zktracer.module.hub.fragment.DomSubStampsSubFragment;
import net.consensys.linea.zktracer.module.hub.fragment.account.AccountFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.ImcFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.MxpCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.StpCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.opcodes.CallOobCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.opcodes.XCallOobCall;
import net.consensys.linea.zktracer.module.hub.fragment.scenario.CallScenarioFragment;
import net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata;
import net.consensys.linea.zktracer.module.hub.section.TraceSection;
import net.consensys.linea.zktracer.module.hub.section.call.precompileSubsection.*;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import net.consensys.linea.zktracer.types.EWord;
import net.consensys.linea.zktracer.types.MemoryRange;
import net.consensys.linea.zktracer.types.Range;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.operation.Operation;
import org.hyperledger.besu.evm.worldstate.WorldUpdater;
import org.hyperledger.besu.evm.worldstate.WorldView;

/**
 * A {@link CallSection} first detects exceptional CALL-type instructions. Exceptional CALL's are
 * easily dealt with and require no post-processing.
 *
 * <p>Unexceptional CALL-type instructions, including aborted ones, <b>always</b> require some
 * degree of post-processing. For one, they are <b>all</b> rollback sensitive as it pertains to
 * value transfers and warmth. As such everything gets scheduled for post rollback.
 *
 * <p>We also need to schedule unexceptional {@link CallSection}'s for post-transaction resolution.
 * Indeed, the following must always be performed, in that order, at transaction end:
 *
 * <p>- append the precompile subsection (if applicable)
 *
 * <p>- append the final context fragment
 */
public class CallSection extends TraceSection
    implements PostOpcodeDefer,
        ContextEntryDefer,
        ContextExitDefer,
        ContextReEntryDefer,
        PostRollbackDefer,
        EndTransactionDefer {

  public static final short NB_ROWS_HUB_CALL = 11; // 2 stack + up to 9 for SMC failure will revert

  public Optional<Address> precompileAddress;

  // row i+0
  private final CallScenarioFragment scenarioFragment = new CallScenarioFragment();

  // last row
  @Setter private ContextFragment finalContextFragment;

  private Address callerAddress;
  private Address calleeAddress;
  private Bytes rawCalleeAddress;
  final ImcFragment firstImcFragment;

  // First couple of rows
  private AccountSnapshot callerFirst;
  private AccountSnapshot calleeFirst;
  private AccountSnapshot callerFirstNew;
  private AccountSnapshot calleeFirstNew;

  // Second couple of rows
  private AccountSnapshot callerSecond;
  private AccountSnapshot calleeSecond;
  private AccountSnapshot callerSecondNew;
  private AccountSnapshot calleeSecondNew;

  // Final row (for scenario/CALL_SMC_FAILURE_WILL_REVERT)
  private AccountSnapshot calleeThird;
  private AccountSnapshot calleeThirdNew;

  // Just after re-entry
  private AccountSnapshot reEntryCallerSnapshot;
  private AccountSnapshot reEntryCalleeSnapshot;

  private final OpCodeData opCode;
  private Wei value;

  public StpCall stpCall;
  private PrecompileSubsection precompileSubsection;

  @Getter private MemoryRange callDataRange;
  @Getter private MemoryRange returnAtRange;

  final Factories factory;

  public CallSection(Hub hub, MessageFrame frame) {
    super(hub, maxNumberOfLines(hub));

    factory = hub.factories();
    opCode = hub.opCodeData();

    final short exceptions = hub.pch().exceptions();

    // row i + 1
    final ContextFragment currentContextFragment = ContextFragment.readCurrentContextData(hub);
    // row i + 2
    firstImcFragment = ImcFragment.empty(hub);

    this.addStackAndFragments(hub, scenarioFragment, currentContextFragment, firstImcFragment);

    if (Exceptions.any(exceptions)) {
      scenarioFragment.setScenario(CALL_EXCEPTION);
      if (opCode.mnemonic() == CALL) {
        firstImcFragment.callOob(new XCallOobCall());
      }
    }

    // STATICX cases
    if (Exceptions.staticFault(exceptions)) {
      return;
    }

    final MxpCall mxpCall = MxpCall.newMxpCall(hub);
    firstImcFragment.callMxp(mxpCall);
    checkArgument(
        mxpCall.mxpx == Exceptions.memoryExpansionException(exceptions),
        "mxp module MXPX does not match the hub's MXPX");

    // MXPX case
    if (Exceptions.memoryExpansionException(exceptions)) {
      return;
    }

    stpCall = new StpCall(hub, frame, mxpCall.gasMxp);
    firstImcFragment.callStp(stpCall);
    checkArgument(
        stpCall.outOfGasException() == Exceptions.outOfGasException(exceptions),
        String.format(
            "The STP and the HUB have conflicting predictions of an OOGX\n\t\tHUB_STAMP = %s",
            hubStamp()));

    final CallFrame currentFrame = hub.currentFrame();
    callerAddress = frame.getRecipientAddress();
    rawCalleeAddress = frame.getStackItem(1);
    calleeAddress = Address.extract(EWord.of(rawCalleeAddress));

    callerFirst = canonical(hub, callerAddress);
    calleeFirst = canonical(hub, calleeAddress);

    // OOGX case
    if (Exceptions.outOfGasException(exceptions)) {
      this.oogXCall();
      return;
    }

    // The CALL is now unexceptional
    checkArgument(Exceptions.none(exceptions), "Unexpected exception in CallSection");
    currentFrame.childSpanningSection(this);

    // the call data span and ``return at'' spans are only required once the CALL is unexceptional
    callDataRange =
        new MemoryRange(
            currentFrame.contextNumber(), Range.callDataRange(frame, hub.opCodeData(frame)), frame);
    returnAtRange = new MemoryRange(currentFrame.contextNumber(), returnAtRange(frame), frame);

    value =
        opCode.callHasValueArgument()
            ? Wei.of(frame.getStackItem(2).toUnsignedBigInteger())
            : Wei.ZERO;

    final CallOobCall oobCall = (CallOobCall) firstImcFragment.callOob(new CallOobCall());

    final boolean aborts = hub.pch().abortingConditions().any();
    checkArgument(
        oobCall.isAbortingCondition() == aborts,
        "oob module ABORT prediction and hub module ABORT prediction mismatch");

    hub.defers().scheduleForPostRollback(this, currentFrame);
    hub.defers().scheduleForEndTransaction(this);

    // The CALL is now unexceptional and un-aborted
    refineUndefinedScenario(hub, frame);
    final CallScenarioFragment.CallScenario scenario = scenarioFragment.getScenario();
    switch (scenario) {
      case CALL_ABORT_WONT_REVERT -> abortingCall(hub);
      case CALL_EOA_UNDEFINED -> eoaProcessing(hub);
      case CALL_SMC_UNDEFINED -> smcProcessing(hub, frame);
      case CALL_PRC_UNDEFINED -> prcProcessing(hub);
      default -> throw new RuntimeException("Illegal CALL scenario");
    }
  }

  private static short maxNumberOfLines(final Hub hub) {
    // 99 % of the time this number of rows will be sufficient
    if (Exceptions.any(hub.pch().exceptions())) {
      return 8;
    }
    if (hub.pch().abortingConditions().any()) {
      return 9;
    }
    return 12; // 12 = 2 (stack) + 5 (CALL prequel) + 5 (successful PRC, except BLAKE and MODEXP)
  }

  private void oogXCall() {

    final AccountFragment callerAccountFragment =
        factory
            .accountFragment()
            .make(
                callerFirst,
                callerFirst,
                DomSubStampsSubFragment.standardDomSubStamps(this.hubStamp(), 0),
                TransactionProcessingType.USER);

    final AccountFragment calleeAccountFragment =
        factory
            .accountFragment()
            .makeWithTrm(
                calleeFirst,
                calleeFirst,
                rawCalleeAddress,
                DomSubStampsSubFragment.standardDomSubStamps(this.hubStamp(), 1),
                TransactionProcessingType.USER);

    this.addFragments(callerAccountFragment, calleeAccountFragment);
  }

  private void abortingCall(Hub hub) {

    callerFirstNew = callerFirst.deepCopy();
    calleeFirstNew = calleeFirst.deepCopy().turnOnWarmth();
    final AccountFragment readingCallerAccount =
        factory
            .accountFragment()
            .make(
                callerFirst,
                callerFirstNew,
                DomSubStampsSubFragment.standardDomSubStamps(this.hubStamp(), 0),
                TransactionProcessingType.USER);

    final AccountFragment readingCalleeAccountAndWarmth =
        factory
            .accountFragment()
            .makeWithTrm(
                calleeFirst,
                calleeFirstNew,
                rawCalleeAddress,
                DomSubStampsSubFragment.standardDomSubStamps(this.hubStamp(), 1),
                TransactionProcessingType.USER);
    finalContextFragment = ContextFragment.nonExecutionProvidesEmptyReturnData(hub);
    this.addFragments(readingCallerAccount, readingCalleeAccountAndWarmth);
    hub.defers().scheduleForPostExecution(this);

    // we immediately reap the call stipend
    commonValues.collectChildStipend(hub);
  }

  /**
   * Sets the scenario to the relevant undefined variant, i.e. either
   *
   * <p>- {@link
   * net.consensys.linea.zktracer.module.hub.fragment.scenario.CallScenarioFragment.CallScenario#CALL_PRC_UNDEFINED}
   *
   * <p>- {@link
   * net.consensys.linea.zktracer.module.hub.fragment.scenario.CallScenarioFragment.CallScenario#CALL_SMC_UNDEFINED}
   *
   * <p>- {@link
   * net.consensys.linea.zktracer.module.hub.fragment.scenario.CallScenarioFragment.CallScenario#CALL_EOA_UNDEFINED}
   *
   * <p>depending on the address.
   *
   * @param hub
   */
  private void refineUndefinedScenario(Hub hub, MessageFrame frame) {

    final boolean aborts = hub.pch().abortingConditions().any();
    if (aborts) {
      scenarioFragment.setScenario(CALL_ABORT_WONT_REVERT);
      return;
    }

    final WorldUpdater world = frame.getWorldUpdater();
    if (isPrecompile(hub.fork, calleeAddress)) {
      precompileAddress = Optional.of(calleeAddress);
      scenarioFragment.setScenario(CALL_PRC_UNDEFINED);
    } else {
      Optional.ofNullable(world.get(calleeAddress))
          .ifPresentOrElse(
              account -> {
                scenarioFragment.setScenario(
                    account.hasCode() ? CALL_SMC_UNDEFINED : CALL_EOA_UNDEFINED);
              },
              () -> {
                scenarioFragment.setScenario(CALL_EOA_UNDEFINED);
              });
    }
  }

  private void eoaProcessing(Hub hub) {
    hub.defers().scheduleForContextReEntry(this, hub.currentFrame());
    commonValues.collectChildStipend(hub);
    finalContextFragment = ContextFragment.nonExecutionProvidesEmptyReturnData(hub);
  }

  private void smcProcessing(Hub hub, MessageFrame frame) {
    final CallFrame currentFrame = hub.currentFrame();
    hub.defers().scheduleForContextEntry(this);
    hub.defers().scheduleForContextExit(this, hub.callStack().futureId());
    hub.defers().scheduleForContextReEntry(this, currentFrame);

    hub.defers().scheduleForContextReEntry(firstImcFragment, currentFrame);

    this.commonValues.payGasPaidOutOfPocket(hub);
    finalContextFragment = ContextFragment.initializeExecutionContext(hub);
    hub.romLex().callRomLex(frame);
  }

  private PrecompileSubsection getPrecompileSubsection(Hub hub) {
    return switch (addressToPrecompileFlag(calleeFirst.address())) {
      case PRC_ECRECOVER,
          PRC_POINT_EVALUATION,
          PRC_BLS_G1_ADD,
          PRC_BLS_G1_MSM,
          PRC_BLS_G2_ADD,
          PRC_BLS_G2_MSM,
          PRC_BLS_PAIRING_CHECK,
          PRC_BLS_MAP_FP_TO_G1,
          PRC_BLS_MAP_FP2_TO_G2,
          PRC_ECADD,
          PRC_ECMUL,
          PRC_ECPAIRING,
          PRC_P256_VERIFY ->
          new EllipticCurvePrecompileSubsection(hub, this);
      case PRC_SHA2_256, PRC_RIPEMD_160 -> new ShaTwoOrRipemdSubSection(hub, this);
      case PRC_IDENTITY -> new IdentitySubsection(hub, this);
      case PRC_MODEXP ->
          new ModexpSubsection(hub, this, new ModexpMetadata(this.getCallDataRange()));
      case PRC_BLAKE2F -> new BlakeSubsection(hub, this);
    };
  }

  private void prcProcessing(Hub hub) {
    precompileSubsection = getPrecompileSubsection(hub);

    hub.defers().scheduleForContextEntry(this);
    hub.defers().scheduleForContextReEntry(this, hub.currentFrame());
  }

  @Override
  public void resolvePostExecution(
      Hub hub, MessageFrame frame, Operation.OperationResult operationResult) {
    // we unlatched the stack after a CALL if and only if we don't "contextEnter" the CALL.
    hub.unlatchStack(frame, this);
  }

  @Override
  public void resolveUponContextEntry(Hub hub, MessageFrame frame) {

    final CallScenarioFragment.CallScenario scenario = scenarioFragment.getScenario();
    checkState(
        scenario == CALL_SMC_UNDEFINED | scenario == CALL_PRC_UNDEFINED,
        String.format(
            "CallSection: call scenario %s should be undefined at context entry resolution",
            scenario));

    callerFirstNew = callerFirst.deepCopy();
    calleeFirstNew = calleeFirst.deepCopy().turnOnWarmth();

    if (opCode.mnemonic() == CALL) {
      callerFirstNew.decrementBalanceBy(value);
      calleeFirstNew.incrementBalanceBy(value);
    }

    // we may be doing more stuff here later
    if (scenarioFragment.getScenario() == CALL_PRC_UNDEFINED) {
      return;
    }

    if (isNonzeroValueSelfCall()) {
      checkState(
          scenarioFragment.getScenario() == CALL_SMC_UNDEFINED,
          "CallSection: self-calls cannot involve precompiles");
      calleeFirst = callerFirstNew.deepCopy();
      calleeFirstNew = callerFirst.deepCopy();
    }

    final AccountFragment firstCallerAccountFragment =
        factory
            .accountFragment()
            .make(
                callerFirst,
                callerFirstNew,
                DomSubStampsSubFragment.standardDomSubStamps(this.hubStamp(), 0),
                TransactionProcessingType.USER);

    final AccountFragment firstCalleeAccountFragment =
        factory
            .accountFragment()
            .makeWithTrm(
                calleeFirst,
                calleeFirstNew,
                rawCalleeAddress,
                DomSubStampsSubFragment.standardDomSubStamps(this.hubStamp(), 1),
                TransactionProcessingType.USER);

    firstCalleeAccountFragment.requiresRomlex(true);

    this.addFragments(firstCallerAccountFragment, firstCalleeAccountFragment);
  }

  /** Resolution happens as the child context is about to terminate. */
  @Override
  public void resolveUponContextExit(Hub hub, CallFrame frame) {
    checkArgument(
        scenarioFragment.getScenario() == CALL_SMC_UNDEFINED,
        "Illegal CALL scenario at context exit");
  }

  @Override
  public void resolveAtContextReEntry(Hub hub, CallFrame frame) {
    // The callSuccess will only be set
    // if the call is acted upon i.e. if the call is un-exceptional and un-aborted
    final boolean success = bytesToBoolean(hub.messageFrame().getStackItem(0));

    reEntryCallerSnapshot = canonical(hub, callerAddress);
    reEntryCalleeSnapshot = canonical(hub, calleeAddress);

    switch (scenarioFragment.getScenario()) {
      case CALL_EOA_UNDEFINED -> {
        checkState(
            success,
            String.format(
                "EOA calls that are still %s at context re-entry cannot fail", CALL_EOA_UNDEFINED));
        scenarioFragment.setScenario(CALL_EOA_SUCCESS_WONT_REVERT);
        firstAccountRowsEoaOrPrc(hub);
        final long gasAfterCall = frame.frame().getRemainingGas();
        commonValues.gasNext(gasAfterCall);
        hub.currentFrame().lastValidGasNext(gasAfterCall);
      }

      case CALL_PRC_UNDEFINED -> {
        scenarioFragment.setScenario(success ? CALL_PRC_SUCCESS_WONT_REVERT : CALL_PRC_FAILURE);
        firstAccountRowsEoaOrPrc(hub);
        final long gasAfterCall = frame.frame().getRemainingGas();
        commonValues.gasNext(gasAfterCall);
        hub.currentFrame().lastValidGasNext(gasAfterCall);

        finalContextFragment =
            ContextFragment.updateReturnData(
                hub, hub.currentFrame(), precompileSubsection.returnDataRange);
      }

      case CALL_SMC_UNDEFINED -> {
        // CALL_SMC_SUCCESS_XXX case
        if (success) {
          scenarioFragment.setScenario(CALL_SMC_SUCCESS_WONT_REVERT);
          return;
        }

        // CALL_SMC_FAILURE_XXX case
        scenarioFragment.setScenario(CALL_SMC_FAILURE_WONT_REVERT);

        callerSecond = callerFirstNew.deepCopy().setDeploymentInfo(hub);
        callerSecondNew = callerFirst.deepCopy().setDeploymentInfo(hub);
        calleeSecond = calleeFirstNew.deepCopy().setDeploymentInfo(hub);
        calleeSecondNew = calleeFirst.deepCopy().setDeploymentInfo(hub).turnOnWarmth();

        final int childId = hub.currentFrame().childFrameIds().getLast();
        final CallFrame childFrame = hub.callStack().getById(childId);
        final int childContextRevertStamp = childFrame.revertStamp();

        final AccountFragment postReEntryCallerAccountFragment =
            factory
                .accountFragment()
                .make(
                    callerSecond,
                    callerSecondNew,
                    DomSubStampsSubFragment.revertsWithChildDomSubStamps(
                        this.hubStamp(), childContextRevertStamp, 2),
                    TransactionProcessingType.USER);

        final AccountFragment postReEntryCalleeAccountFragment =
            factory
                .accountFragment()
                .make(
                    calleeSecond,
                    calleeSecondNew,
                    DomSubStampsSubFragment.revertsWithChildDomSubStamps(
                        this.hubStamp(), childContextRevertStamp, 3),
                    TransactionProcessingType.USER);

        this.addFragments(postReEntryCallerAccountFragment, postReEntryCalleeAccountFragment);
      }

      default -> throw new IllegalArgumentException("Illegal CALL scenario");
    }
  }

  @Override
  public void resolveUponRollback(Hub hub, MessageFrame messageFrame, CallFrame callFrame) {

    final CallScenarioFragment.CallScenario callScenario = scenarioFragment.getScenario();
    switch (callScenario) {
      case CALL_ABORT_WONT_REVERT -> completeAbortWillRevert(hub);
      case CALL_EOA_SUCCESS_WONT_REVERT -> completeEoaSuccessWillRevert(hub);
      case CALL_SMC_FAILURE_WONT_REVERT -> completeSmcFailureWillRevert(hub);
      case CALL_SMC_SUCCESS_WONT_REVERT, CALL_PRC_SUCCESS_WONT_REVERT ->
          completeSmcOrPrcSuccessWillRevert(hub);
      case CALL_PRC_FAILURE -> {
        // Note: no undoing required
        //  - account snapshots were taken with value transfers undone
        //  - precompiles are warm by definition so no warmth undoing required
        return;
      }
      default -> throw new IllegalArgumentException("Illegal CALL scenario");
    }
  }

  @Override
  public void resolveAtEndTransaction(
      Hub hub, WorldView state, Transaction tx, boolean isSuccessful) {

    final CallScenarioFragment.CallScenario scenario = scenarioFragment.getScenario();

    checkArgument(
        scenario.noLongerUndefined(),
        String.format(
            "Call scenario = %s, HUB_STAMP = %s, successBit = %s",
            scenarioFragment.getScenario(), this.hubStamp(), isSuccessful));

    if (scenario.isPrcCallScenario()) {
      this.addFragments(precompileSubsection.fragments());
    }

    this.addFragment(finalContextFragment);
  }

  private void completeAbortWillRevert(Hub hub) {
    scenarioFragment.setScenario(CALL_ABORT_WILL_REVERT);
    calleeSecond = calleeFirstNew.deepCopy().setDeploymentInfo(hub);
    calleeSecondNew = calleeFirst.deepCopy().setDeploymentInfo(hub);
    final AccountFragment undoingCalleeAccountFragment =
        factory
            .accountFragment()
            .make(
                calleeSecond,
                calleeSecondNew,
                DomSubStampsSubFragment.revertWithCurrentDomSubStamps(
                    this.hubStamp(), this.revertStamp(), 2),
                TransactionProcessingType.USER);
    this.addFragment(undoingCalleeAccountFragment);
  }

  private void completeEoaSuccessWillRevert(Hub hub) {
    scenarioFragment.setScenario(CALL_EOA_SUCCESS_WILL_REVERT);

    callerSecond = reEntryCallerSnapshot.deepCopy().setDeploymentNumber(hub);
    callerSecondNew = callerFirst.deepCopy().setDeploymentNumber(hub);

    calleeSecond = reEntryCalleeSnapshot.deepCopy().setDeploymentNumber(hub);
    calleeSecondNew = calleeFirst.deepCopy().setDeploymentNumber(hub);

    final AccountFragment undoingCallerAccountFragment =
        factory
            .accountFragment()
            .make(
                callerSecond,
                callerSecondNew,
                DomSubStampsSubFragment.revertWithCurrentDomSubStamps(
                    this.hubStamp(), this.revertStamp(), 2),
                TransactionProcessingType.USER);

    final AccountFragment undoingCalleeAccountFragment =
        factory
            .accountFragment()
            .make(
                calleeSecond,
                calleeSecondNew,
                DomSubStampsSubFragment.revertWithCurrentDomSubStamps(
                    this.hubStamp(), this.revertStamp(), 3),
                TransactionProcessingType.USER);

    this.addFragments(undoingCallerAccountFragment, undoingCalleeAccountFragment);
  }

  private void completeSmcFailureWillRevert(Hub hub) {
    scenarioFragment.setScenario(CALL_SMC_FAILURE_WILL_REVERT);

    if (isSelfCall()) {
      calleeThird = callerSecondNew.deepCopy().setDeploymentNumber(hub);
      calleeThirdNew = callerFirst.deepCopy().setDeploymentNumber(hub);
    } else {
      calleeThird = calleeSecondNew.deepCopy().setDeploymentNumber(hub);
      calleeThirdNew = calleeFirst.deepCopy().setDeploymentNumber(hub);
    }

    // this (should) work for both self calls and foreign address calls
    final AccountFragment undoingCalleeWarmthAccountFragment =
        factory
            .accountFragment()
            .make(
                calleeThird,
                calleeThirdNew,
                DomSubStampsSubFragment.revertWithCurrentDomSubStamps(
                    this.hubStamp(), this.revertStamp(), 4),
                TransactionProcessingType.USER);

    this.addFragment(undoingCalleeWarmthAccountFragment);
  }

  private void completeSmcOrPrcSuccessWillRevert(Hub hub) {

    final CallScenarioFragment.CallScenario callScenario = scenarioFragment.getScenario();
    checkState(
        callScenario.isAnyOf(CALL_SMC_SUCCESS_WONT_REVERT, CALL_PRC_SUCCESS_WONT_REVERT),
        "The only CALL scenarios that can be successful and reverted (down stream) are %s and %s, yet we are given %s",
        CALL_SMC_SUCCESS_WONT_REVERT,
        CALL_PRC_SUCCESS_WONT_REVERT,
        callScenario);
    if (callScenario == CALL_SMC_SUCCESS_WONT_REVERT) {
      scenarioFragment.setScenario(CALL_SMC_SUCCESS_WILL_REVERT);
    } else {
      scenarioFragment.setScenario(CALL_PRC_SUCCESS_WILL_REVERT);
    }

    callerSecond = callerFirstNew.deepCopy().setDeploymentNumber(hub);
    callerSecondNew = callerFirst.deepCopy().setDeploymentNumber(hub);

    calleeSecond = calleeFirstNew.deepCopy().setDeploymentNumber(hub);
    calleeSecondNew = calleeFirst.deepCopy().setDeploymentNumber(hub);

    final AccountFragment undoingCallerAccountFragment =
        factory
            .accountFragment()
            .make(
                callerSecond,
                callerSecondNew,
                DomSubStampsSubFragment.revertWithCurrentDomSubStamps(
                    this.hubStamp(), this.revertStamp(), 2),
                TransactionProcessingType.USER);
    final AccountFragment undoingCalleeAccountFragment =
        factory
            .accountFragment()
            .make(
                calleeSecond,
                calleeSecondNew,
                DomSubStampsSubFragment.revertWithCurrentDomSubStamps(
                    this.hubStamp(), this.revertStamp(), 3),
                TransactionProcessingType.USER);

    this.addFragments(undoingCallerAccountFragment, undoingCalleeAccountFragment);
  }

  private void firstAccountRowsEoaOrPrc(final Hub hub) {

    callerFirstNew = canonical(hub, callerAddress);
    calleeFirstNew = canonical(hub, calleeAddress);

    final AccountFragment firstCallerAccountFragment =
        factory
            .accountFragment()
            .make(
                callerFirst,
                callerFirstNew,
                DomSubStampsSubFragment.standardDomSubStamps(this.hubStamp(), 0),
                TransactionProcessingType.USER);

    final AccountFragment firstCalleeAccountFragment =
        factory
            .accountFragment()
            .makeWithTrm(
                calleeFirst,
                calleeFirstNew,
                rawCalleeAddress,
                DomSubStampsSubFragment.standardDomSubStamps(this.hubStamp(), 1),
                TransactionProcessingType.USER);

    this.addFragments(firstCallerAccountFragment, firstCalleeAccountFragment);
  }

  private Range returnAtRange(MessageFrame frame) {
    final Bytes returnAtCapacity =
        opCode.callHasValueArgument() ? frame.getStackItem(6) : frame.getStackItem(5);
    final Bytes returnAtOffset =
        opCode.callHasValueArgument() ? frame.getStackItem(5) : frame.getStackItem(4);

    return Range.fromOffsetAndSize(returnAtOffset, returnAtCapacity);
  }

  private boolean isSelfCall() {
    checkState(
        scenarioFragment.getScenario().isIndefiniteSmcCallScenario(),
        "self-calls only make sense for SMC call scenarios");
    return calleeAddress.equals(callerAddress);
  }

  private boolean isNonzeroValueSelfCall() {
    checkState(
        scenarioFragment.getScenario().isIndefiniteSmcCallScenario(),
        "(nonzero value) self-calls only make sense for SMC call scenarios");
    return isSelfCall() && !value.isZero();
  }
}
