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

package net.consensys.linea.zktracer.module.hub.fragment.common;

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.Trace.EVM_INST_PUSH0;
import static net.consensys.linea.zktracer.module.hub.HubProcessingPhase.TX_EXEC;
import static net.consensys.linea.zktracer.module.hub.TransactionProcessingType.USER;
import static net.consensys.linea.zktracer.module.hub.signals.Exceptions.*;
import static net.consensys.linea.zktracer.module.hub.signals.TracedException.*;
import static net.consensys.linea.zktracer.module.hub.signals.TracedException.MAX_CODE_SIZE_EXCEPTION;
import static net.consensys.linea.zktracer.opcode.InstructionFamily.*;

import java.math.BigInteger;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.Setter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.HubProcessingPhase;
import net.consensys.linea.zktracer.module.hub.TransactionProcessingType;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import net.consensys.linea.zktracer.module.hub.signals.TracedException;
import net.consensys.linea.zktracer.module.hub.state.State;
import net.consensys.linea.zktracer.opcode.InstructionFamily;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.opcode.gas.projector.GasProjection;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import net.consensys.linea.zktracer.runtime.callstack.CallStack;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;

@Accessors(fluent = true)
@RequiredArgsConstructor
public class CommonFragmentValues {
  public final Hub hub;
  public final TransactionProcessingMetadata txMetadata;
  @Getter final short relBlockNumber;
  @Getter final short sysiTransactionNumber;
  @Getter final short userTransactionNumber;
  @Getter final short sysfTransactionNumber;
  public final HubProcessingPhase hubProcessingPhase;
  public final TransactionProcessingType transactionProcessingType;
  public final int hubStamp;
  public final CallStack callStack;
  public final State.HubTransactionState.Stamps stamps;
  @Setter public int logStamp = -1;
  @Getter final CallFrame callFrame;
  public final short exceptions;
  public final boolean contextMayChange;
  @Setter public int contextNumberNew;
  public final int pc;
  public final int pcNew;
  final short height;
  final short heightNew;
  public final long gasExpected;
  public final long gasActual;
  @Getter final long gasCost;
  @Getter @Setter long gasNext;
  @Getter final long gasCostExcluduingDeploymentCost;
  @Setter public long refundDelta = 0; // 0 is default Value, can be modified only by SSTORE section
  @Setter public long gasRefund; // Set at commit time
  @Setter public long gasRefundNew; // Set at commit time
  @Setter public int numberOfNonStackRows;
  @Setter public boolean TLI;
  @Setter public int codeFragmentIndex = -1;
  @Getter private TracedException tracedException = UNDEFINED;

  public CommonFragmentValues(Hub hub) {
    final short exceptions = hub.pch().exceptions();
    final boolean stackException = stackException(exceptions);

    final boolean isExec = hub.state.processingPhase() == TX_EXEC;
    final OpCodeData opCode = hub.opCodeData();

    this.hub = hub;
    relBlockNumber = (short) hub.blockStack().currentRelativeBlockNumber();
    sysiTransactionNumber = hub.state().sysiTransactionNumber();
    userTransactionNumber = hub.state.getUserTransactionNumber();
    sysfTransactionNumber = hub.state().sysfTransactionNumber();
    this.hubProcessingPhase = hub.state().processingPhase();
    transactionProcessingType = hub.state.transactionProcessingType();
    txMetadata = transactionProcessingType == USER ? hub.txStack().current() : null;
    this.hubStamp = hub.stamp();
    this.callStack = hub.callStack();
    this.stamps = hub.state().stamps();
    this.callFrame = hub.currentFrame();
    this.exceptions = exceptions;
    this.pc = isExec ? callFrame.pc() : 0;
    this.pcNew = computePcNew(callFrame, opCode, pc, stackException, isExec);
    this.height = callFrame.stack().getHeight();
    this.heightNew = callFrame.stack().getHeightNew();

    this.gasExpected = computeGasExpected(callFrame);
    this.gasActual = hub.remainingGas();
    final GasProjection op = isExec ? hub.gasProjector.of(hub.messageFrame(), opCode) : null;
    this.gasCost = isExec ? op.upfrontGasCost() : 0;
    this.gasNext = isExec ? computeGasNext(exceptions) : 0;
    this.gasCostExcluduingDeploymentCost = isExec ? op.gasCostExcludingDeploymentCost() : 0;

    final InstructionFamily instructionFamily = opCode.instructionFamily();
    this.contextMayChange =
        isExec
            && ((instructionFamily == CALL
                    || instructionFamily == CREATE
                    || instructionFamily == HALT
                    || instructionFamily == INVALID)
                || any(this.exceptions));

    if (contextMayChange) {
      // Trigger the gas module in case contextMayChange is true
      hub.gas().call(hub, this);
    }

    if (none(exceptions)) {
      tracedException = TracedException.NONE;
      return;
    }

    if (Exceptions.staticFault(exceptions)) {
      checkArgument(
          opCode.mayTriggerStaticException(),
          "CommonFragmentValues: opCode %s throws impossible static exception",
          opCode);
      setTracedException(TracedException.STATIC_FAULT);
      return;
    }

    // RETURNDATACOPY opcode specific exception
    if (opCode.mnemonic() == OpCode.RETURNDATACOPY && Exceptions.returnDataCopyFault(exceptions)) {
      setTracedException(TracedException.RETURN_DATA_COPY_FAULT);
      return;
    }

    // SSTORE opcode specific exception
    if (opCode.mnemonic() == OpCode.SSTORE && Exceptions.outOfSStore(exceptions)) {
      setTracedException(TracedException.OUT_OF_SSTORE);
      return;
    }

    // For RETURN, in case none of the above exceptions is the traced one,
    // we have a complex logic to determine the traced exception that is
    // implemented in the instruction processing
    if (opCode.mnemonic() == OpCode.RETURN) {
      return;
    }

    if (maxCodeSizeException(exceptions))
    // the MaxCodeSize exceptions for return is already dealt before
    {
      checkArgument(opCode.isCreate(), "MaxCodeSize exception on non CREATE opcode" + opCode);
      setTracedException(MAX_CODE_SIZE_EXCEPTION);
      return;
    }

    if (Exceptions.memoryExpansionException(exceptions)) {
      checkArgument(
          opCode.mayTriggerMemoryExpansionException(), "MXP triggered by non MXP opcode" + opCode);
      setTracedException(TracedException.MEMORY_EXPANSION_EXCEPTION);
      return;
    }

    if (Exceptions.outOfGasException(exceptions)) {
      setTracedException(TracedException.OUT_OF_GAS_EXCEPTION);
      return;
    }

    // JUMP instruction family specific exception
    if (instructionFamily == InstructionFamily.JUMP && Exceptions.jumpFault(exceptions)) {
      setTracedException(TracedException.JUMP_FAULT);
    }
  }

  public void setTracedException(TracedException tracedException) {
    checkArgument(
        this.tracedException == UNDEFINED,
        "Traced exception already set to " + this.tracedException);
    this.tracedException = tracedException;
  }

  static int computePcNew(
      final CallFrame callFrame,
      OpCodeData opCode,
      final int pc,
      boolean stackException,
      boolean isExec) {
    if (!isExec || stackException) {
      return 0;
    }

    // General Case:
    if (!opCode.isNonTrivialPush() && !opCode.isJump()) return pc + 1;

    // PUSHX (with X !=0 ) case
    if (opCode.isNonTrivialPush()) {
      return pc + 1 + (opCode.byteValue() - EVM_INST_PUSH0);
    }

    // JUMP case
    if (opCode.isJump()) {
      final BigInteger prospectivePcNew = callFrame.frame().getStackItem(0).toUnsignedBigInteger();
      final BigInteger codeSize = BigInteger.valueOf(callFrame.code().getSize());

      final int attemptedPcNew =
          codeSize.compareTo(prospectivePcNew) > 0 ? prospectivePcNew.intValueExact() : 0;

      if (opCode.mnemonic().equals(OpCode.JUMP)) {
        return attemptedPcNew;
      }

      if (opCode.mnemonic().equals(OpCode.JUMPI)) {
        final BigInteger condition = callFrame.frame().getStackItem(1).toUnsignedBigInteger();
        if (!condition.equals(BigInteger.ZERO)) {
          return attemptedPcNew;
        } else {
          return pc + 1;
        }
      }
    }

    throw new RuntimeException(
        "Instruction not covered " + opCode.mnemonic() + " unable to compute pcNew.");
  }

  public long computeGasExpected(CallFrame currentFrame) {
    if (hub.state().processingPhase() != TX_EXEC) return 0;

    if (currentFrame.executionPaused()) {
      currentFrame.unpauseCurrentFrame();
      return currentFrame.lastValidGasNext();
    }

    return currentFrame.frame().getRemainingGas();
  }

  /**
   * Returns the value of the GAS_NEXT column. For CALL's and CREATE's it returns
   *
   * <p><center><b>remainingGas - upfrontGasCost</b></center>
   *
   * <p>This initial computation has to be amended down the line to account for
   *
   * <p>- {@link GasProjection#stipend()} for aborted calls / EOA calls
   *
   * <p>- {@link GasProjection#gasPaidOutOfPocket()} when entering a <b>CALL</b>/<b>CREATE</b>
   *
   * <p>- precompile specific costs for PRC calls
   *
   * <p>The stipend is done through {@link CommonFragmentValues#collectChildStipend(Hub)}}
   *
   * @param exceptions
   * @return
   */
  public long computeGasNext(short exceptions) {
    return Exceptions.any(exceptions) ? 0 : gasActual - gasCost;
    // Note:
    //   for CREATE(s) -> this is only part of the story because of
    //     //  1. nonempty init code CREATE's where gas is paid out of pocket
    //     // This is done in the CREATE section
    //   for CALL(s) -> this is only part of the story because of
    //     //  1. aborts with value transfers (immediately reapStipend)
    //     //  2. EOA calls with value transfer (immediately reapStipend)
    //     //  3. SMC calls: gas paid out of pocket
    //     //  4. PRC calls: gas paid out of pocket + special PRC cost + returned gas
    //     // This is done in the CALL section
  }

  public void payGasPaidOutOfPocket(Hub hub) {
    this.gasNext -= hub.gasProjector.of(hub.messageFrame(), hub.opCodeData()).gasPaidOutOfPocket();
  }

  public void collectChildStipend(Hub hub) {
    this.gasNext += hub.gasProjector.of(hub.messageFrame(), hub.opCodeData()).stipend();
  }

  public long gasCostToTrace() {

    if (hubProcessingPhase != TX_EXEC
        || tracedException() == TracedException.STACK_UNDERFLOW
        || tracedException() == TracedException.STACK_OVERFLOW
        || tracedException() == TracedException.RETURN_DATA_COPY_FAULT
        || tracedException() == TracedException.MEMORY_EXPANSION_EXCEPTION
        || tracedException() == TracedException.STATIC_FAULT
        || tracedException() == TracedException.INVALID_CODE_PREFIX
        || tracedException() == MAX_CODE_SIZE_EXCEPTION) {
      return 0;
    }

    return gasCost;
  }
}
