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

import static com.google.common.base.Preconditions.checkArgument;
import static com.google.common.base.Preconditions.checkState;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.ReturnScenarioFragment.ReturnScenario.*;
import static net.consensys.linea.zktracer.module.hub.signals.Exceptions.OUT_OF_GAS_EXCEPTION;
import static net.consensys.linea.zktracer.module.hub.signals.Exceptions.memoryExpansionException;
import static org.hyperledger.besu.evm.frame.MessageFrame.Type.*;

import lombok.Getter;
import net.consensys.linea.zktracer.module.hub.AccountSnapshot;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.TransactionProcessingType;
import net.consensys.linea.zktracer.module.hub.defer.ContextReEntryDefer;
import net.consensys.linea.zktracer.module.hub.defer.EndTransactionDefer;
import net.consensys.linea.zktracer.module.hub.defer.PostRollbackDefer;
import net.consensys.linea.zktracer.module.hub.fragment.ContextFragment;
import net.consensys.linea.zktracer.module.hub.fragment.DomSubStampsSubFragment;
import net.consensys.linea.zktracer.module.hub.fragment.account.AccountFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.ImcFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.MxpCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.opcode.ReturnFromDeploymentMmuCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.opcodes.DeploymentOobCall;
import net.consensys.linea.zktracer.module.hub.fragment.scenario.ReturnScenarioFragment;
import net.consensys.linea.zktracer.module.hub.section.TraceSection;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import net.consensys.linea.zktracer.module.hub.signals.TracedException;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import net.consensys.linea.zktracer.types.Bytecode;
import net.consensys.linea.zktracer.types.MemoryRange;
import net.consensys.linea.zktracer.types.Range;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.internal.Words;
import org.hyperledger.besu.evm.worldstate.WorldView;

@Getter
public class ReturnSection extends TraceSection
    implements ContextReEntryDefer, PostRollbackDefer, EndTransactionDefer {

  final boolean returnFromMessageCall;
  final boolean returnFromDeployment;
  boolean nonemptyByteCode;
  final ReturnScenarioFragment returnScenarioFragment;
  AccountFragment deploymentFragment;

  AccountSnapshot firstCreatee;
  AccountSnapshot firstCreateeNew;
  AccountSnapshot secondCreatee;
  AccountSnapshot secondCreateeNew;
  ContextFragment squashParentContextReturnData;
  Address deploymentAddress;

  int hubStamp;

  boolean successfulMessageCallExpected = false; // for sanity check
  boolean successfulDeploymentExpected = false; // for sanity check

  public ReturnSection(Hub hub, MessageFrame frame) {
    super(hub, maxNumberOfRows(hub));

    hubStamp = hub.stamp();
    final CallFrame callFrame = hub.currentFrame();

    returnFromMessageCall = frame.getType().equals(MESSAGE_CALL);
    returnFromDeployment = frame.getType().equals(CONTRACT_CREATION);

    checkArgument(
        callFrame.isDeployment() == (frame.getType().equals(CONTRACT_CREATION)),
        "ReturnSection: CallFrame and MessageFrame inconsistent on whether frame is deployment or not");

    checkArgument(
        returnFromDeployment
            == hub.transients()
                .conflation()
                .deploymentInfo()
                .getDeploymentStatus(frame.getContractAddress()),
        "ReturnSection: Frame and RomLex disagree whether frame is deployment or not, for RomLex that's via the deployment status of the contract address");

    returnScenarioFragment = new ReturnScenarioFragment();
    final ContextFragment currentContextFragment = ContextFragment.readCurrentContextData(hub);
    final ImcFragment firstImcFragment = ImcFragment.empty(hub);
    final MxpCall mxpCall = MxpCall.newMxpCall(hub);
    firstImcFragment.callMxp(mxpCall);

    this.addStackAndFragments(
        hub, returnScenarioFragment, currentContextFragment, firstImcFragment);

    final short exceptions = hub.pch().exceptions();

    if (Exceptions.any(exceptions)) {
      returnScenarioFragment.setScenario(RETURN_EXCEPTION);
    }

    checkArgument(
        mxpCall.mxpx == memoryExpansionException(exceptions),
        "RETURN: mxp and hub disagree on MXPX");

    if (mxpCall.mxpx) {
      commonValues.setTracedException(TracedException.MEMORY_EXPANSION_EXCEPTION);
      return;
    }

    // Note: in case of returnFromMessageCall, we check for outOfGasException.
    // In case of returnFromDeployment, we check for maxCodeSize & invalidCodePrefixException before
    // OOGX.
    if (Exceptions.outOfGasException(exceptions) && returnFromMessageCall) {
      checkArgument(
          exceptions == OUT_OF_GAS_EXCEPTION,
          "RETURN from message call: last exception should be OOGX");
      commonValues.setTracedException(TracedException.OUT_OF_GAS_EXCEPTION);
      return;
    }

    if (Exceptions.any(exceptions)) {
      checkArgument(
          returnFromDeployment,
          "RETURN from message call: exceptions should have already been handled");
    }

    // maxCodeSizeException case
    final boolean triggerOobForMaxCodeSizeException = Exceptions.maxCodeSizeException(exceptions);
    if (triggerOobForMaxCodeSizeException) {
      firstImcFragment.callOob(new DeploymentOobCall());
      commonValues.setTracedException(TracedException.MAX_CODE_SIZE_EXCEPTION);
      return;
    }

    // invalidCodePrefixException case
    final boolean nontrivialMmuOperation = mxpCall.mayTriggerNontrivialMmuOperation;
    final boolean triggerMmuForInvalidCodePrefix = Exceptions.invalidCodePrefix(exceptions);
    if (triggerMmuForInvalidCodePrefix) {
      checkArgument(
          returnFromDeployment && nontrivialMmuOperation,
          "invalidCodePrefixException triggered yet returnFromDeployment = %s and nontrivialMmuOperation = %s",
          returnFromDeployment,
          nontrivialMmuOperation);

      final MmuCall actuallyInvalidCodePrefixMmuCall = MmuCall.invalidCodePrefix(hub);
      firstImcFragment.callMmu(actuallyInvalidCodePrefixMmuCall);

      checkArgument(
          actuallyInvalidCodePrefixMmuCall.successBit(),
          "RETURN from deployment: invalidCodePrefixException incorrectly picked up by MMU");
      commonValues.setTracedException(TracedException.INVALID_CODE_PREFIX);
      return;
    }

    // OOGX case
    if (Exceptions.outOfGasException(exceptions) && returnFromDeployment) {
      checkArgument(
          exceptions == OUT_OF_GAS_EXCEPTION,
          "RETURN from deployment: last exception should be OOGX");
      commonValues.setTracedException(TracedException.OUT_OF_GAS_EXCEPTION);
      return;
    }

    // Unexceptional RETURN's
    // (we have exceptions ≡ ∅ by the checkArgument below)
    //////////////////////////////////////////////////////
    checkArgument(Exceptions.none(exceptions), "RETURN: unexpected exception");

    // RETURN_FROM_MESSAGE_CALL cases
    if (returnFromMessageCall) {
      successfulMessageCallExpected = true;
      final boolean messageCallReturnTouchesRam =
          !callFrame.isRoot()
              && nontrivialMmuOperation // [size ≠ 0] ∧ ¬MXPX
              && !callFrame.returnAtRange().isEmpty(); // [r@c ≠ 0]

      returnScenarioFragment.setScenario(
          messageCallReturnTouchesRam
              ? RETURN_FROM_MESSAGE_CALL_WILL_TOUCH_RAM
              : RETURN_FROM_MESSAGE_CALL_WONT_TOUCH_RAM);

      final Bytes offset = frame.getStackItem(0);
      final Bytes size = frame.getStackItem(1);
      callFrame.outputDataRange(
          new MemoryRange(callFrame.contextNumber(), Range.fromOffsetAndSize(offset, size), frame));

      if (messageCallReturnTouchesRam) {
        final MmuCall returnFromMessageCall = MmuCall.returnFromMessageCall(hub);
        firstImcFragment.callMmu(returnFromMessageCall);
      }

      final ContextFragment updateCallerReturnData =
          ContextFragment.executionProvidesReturnData(hub);
      this.addFragment(updateCallerReturnData);

      return;
    }

    // RETURN_FROM_DEPLOYMENT cases
    if (returnFromDeployment) {
      successfulDeploymentExpected = true;

      // Note: special care when "re-entering" the root of a contract creation parent context as no
      // such context exist
      hub.defers()
          .scheduleForContextReEntry(
              this, hub.callStack().parentCallFrame()); // post deployment account snapshot
      hub.defers().scheduleForPostRollback(this, callFrame); // undo deployment
      hub.defers().scheduleForEndTransaction(this); // inserting the final context row;

      squashParentContextReturnData = ContextFragment.executionProvidesEmptyReturnData(hub);
      deploymentAddress = frame.getRecipientAddress();
      nonemptyByteCode = mxpCall.mayTriggerNontrivialMmuOperation;

      firstCreatee = AccountSnapshot.canonical(hub, frame.getWorldUpdater(), deploymentAddress);

      returnScenarioFragment.setScenario(
          nonemptyByteCode
              ? RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WONT_REVERT
              : RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WONT_REVERT);

      final Bytes byteCodeSize = frame.getStackItem(1);
      checkArgument(
          nonemptyByteCode == (!byteCodeSize.isZero()),
          "RETURN from deployment: mxp nonemptyByteCode = %s disagrees with byteCoedeSize = %s",
          nonemptyByteCode,
          byteCodeSize);

      // Empty deployments
      if (!nonemptyByteCode) {
        if (hub.messageFrame().getDepth() == 0) {
          this.addDeploymentAccountFragmentIfRoot(hub, mxpCall);
        }
        return;
      }

      hub.romLex().callRomLex(frame);

      final MmuCall invalidCodePrefixCheckMmuCall = MmuCall.invalidCodePrefix(hub);
      firstImcFragment.callMmu(invalidCodePrefixCheckMmuCall);

      final DeploymentOobCall maxCodeSizeOobCall =
          (DeploymentOobCall) firstImcFragment.callOob(new DeploymentOobCall());

      // sanity checks
      checkArgument(
          !invalidCodePrefixCheckMmuCall.successBit(),
          "RETURN shouldn't throw invalidCodePrefixException at this stage, but does");
      checkArgument(
          !maxCodeSizeOobCall.isMaxCodeSizeException(),
          "RETURN shouldn't throw maxCodeSizeException at this stage, but does");

      final ImcFragment secondImcFragment = ImcFragment.empty(hub);
      this.addFragment(secondImcFragment);

      final ReturnFromDeploymentMmuCall nonemptyDeploymentMmuCall =
          MmuCall.returnFromDeployment(hub);
      secondImcFragment.callMmu(nonemptyDeploymentMmuCall);

      writeHashInfoResult(nonemptyDeploymentMmuCall.hashResult());

      if (hub.messageFrame().getDepth() == 0) {
        this.addDeploymentAccountFragmentIfRoot(hub, mxpCall);
      }
    }
  }

  @Override
  public void resolveAtContextReEntry(Hub hub, CallFrame frame) {

    checkState(
        returnFromDeployment,
        "ReturnSection: a RETURN requires resolution at context re-entry only for deployments");

    firstCreateeNew = AccountSnapshot.canonical(hub, deploymentAddress);
    final AccountFragment deploymentAccountFragment =
        hub.factories()
            .accountFragment()
            .make(
                firstCreatee,
                firstCreateeNew,
                DomSubStampsSubFragment.standardDomSubStamps(hubStamp, 0),
                TransactionProcessingType.USER);

    if (nonemptyByteCode) {
      deploymentAccountFragment.requiresRomlex(true);
    }

    this.addFragment(deploymentAccountFragment);
  }

  @Override
  public void resolveUponRollback(Hub hub, MessageFrame messageFrame, CallFrame callFrame) {

    checkArgument(returnFromDeployment, "rollback sensitive RETURNs should stem from deployments");
    returnScenarioFragment.setScenario(
        nonemptyByteCode
            ? RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WILL_REVERT
            : RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WILL_REVERT);

    secondCreatee = firstCreateeNew.deepCopy().setDeploymentNumber(hub);
    secondCreateeNew = firstCreatee.deepCopy().setDeploymentNumber(hub);

    final AccountFragment undoingDeploymentAccountFragment =
        hub.factories()
            .accountFragment()
            .make(
                firstCreateeNew,
                secondCreateeNew,
                DomSubStampsSubFragment.revertWithCurrentDomSubStamps(
                    hubStamp, hub.callStack().currentCallFrame().revertStamp(), 1),
                TransactionProcessingType.USER);

    this.addFragment(undoingDeploymentAccountFragment);
  }

  @Override
  public void resolveAtEndTransaction(
      Hub hub, WorldView state, Transaction tx, boolean isSuccessful) {

    checkArgument(
        returnFromDeployment,
        "RETURNs which trigger resolveAtEndTransaction should stem from deployments");
    this.addFragment(squashParentContextReturnData);
  }

  private void addDeploymentAccountFragmentIfRoot(Hub hub, MxpCall mxpCall) {

    checkState(
        returnFromDeployment,
        "RETURN associated addDeploymentAccountFragmentIfRoot should be called only for deployments");

    firstCreateeNew = AccountSnapshot.canonical(hub, deploymentAddress);
    firstCreateeNew.code(
        new Bytecode(
            hub.messageFrame()
                .shadowReadMemory(
                    Words.clampedToLong(mxpCall.offset1), Words.clampedToLong(mxpCall.size1))));
    firstCreateeNew.deploymentStatus(false);
    final AccountFragment deploymentAccountFragment =
        hub.factories()
            .accountFragment()
            .make(
                firstCreatee,
                firstCreateeNew,
                DomSubStampsSubFragment.standardDomSubStamps(hubStamp, 0),
                TransactionProcessingType.USER);

    if (nonemptyByteCode) {
      deploymentAccountFragment.requiresRomlex(true);
    }

    this.addFragment(deploymentAccountFragment);
  }

  private static short maxNumberOfRows(Hub hub) {
    return (short)
        (hub.opCodeData().numberOfStackRows() + (Exceptions.any(hub.pch().exceptions()) ? 4 : 7));
  }
}
