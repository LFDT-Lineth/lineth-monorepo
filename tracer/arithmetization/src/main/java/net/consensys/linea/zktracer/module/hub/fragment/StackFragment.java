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

package net.consensys.linea.zktracer.module.hub.fragment;

import static com.google.common.base.Preconditions.*;
import static com.google.common.primitives.Ints.min;
import static net.consensys.linea.zktracer.Trace.EVM_INST_PUSH0;
import static net.consensys.linea.zktracer.module.hub.signals.TracedException.*;
import static net.consensys.linea.zktracer.opcode.InstructionFamily.*;
import static net.consensys.linea.zktracer.types.Utils.rightPadTo;

import java.math.BigInteger;
import java.util.List;
import java.util.function.Function;
import lombok.Getter;
import lombok.Setter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.common.CommonFragmentValues;
import net.consensys.linea.zktracer.module.hub.signals.AbortingConditions;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import net.consensys.linea.zktracer.module.hub.signals.TracedException;
import net.consensys.linea.zktracer.opcode.InstructionFamily;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.opcode.gas.projector.GasProjection;
import net.consensys.linea.zktracer.runtime.stack.Stack;
import net.consensys.linea.zktracer.runtime.stack.StackItem;
import net.consensys.linea.zktracer.types.EWord;
import net.consensys.linea.zktracer.types.UnsignedByte;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.evm.internal.Words;

@Accessors(fluent = true)
public final class StackFragment implements TraceFragment {
  private final Stack stack;
  @Getter private final List<StackItem> stackOps;
  private final short exceptions;
  private final long staticGas;
  @Setter public boolean hashInfoFlag;
  private EWord hashInfoKeccak = EWord.ZERO;
  @Setter public Bytes hash;
  @Getter private final OpCodeData opCode;
  @Getter private final int rawOpCode;
  @Setter private boolean jumpDestinationVettingRequired;
  private final CommonFragmentValues commonFragmentValues;
  private final EWord pushValue;

  private StackFragment(
      final Hub hub,
      Stack stack,
      List<StackItem> stackOps,
      short exceptions,
      AbortingConditions aborts,
      GasProjection gp,
      boolean isDeploying,
      CommonFragmentValues commonFragmentValues) {
    this.stack = stack;
    this.stackOps = stackOps;
    this.exceptions = exceptions;
    this.opCode = stack.getCurrentOpcodeData();
    if (this.opCode.mnemonic() != OpCode.INVALID) {
      this.rawOpCode = 0xff & this.opCode.byteValue();
    } else {
      final int codeSize = hub.messageFrame().getCode().getBytes().size();
      final int pc = hub.messageFrame().getPC();
      this.rawOpCode = (pc < codeSize) ? 0xff & hub.messageFrame().getCode().getBytes().get(pc) : 0;
    }
    this.hashInfoFlag =
        switch (this.opCode.mnemonic()) {
              case SHA3 -> true;
              case RETURN -> isDeploying;
              case CREATE2 -> aborts.none();
              default -> false;
            }
            && Exceptions.none(exceptions)
            && gp.messageSize() > 0;
    if (this.hashInfoFlag) {
      Bytes memorySegmentToHash;
      switch (this.opCode.mnemonic()) {
        case SHA3, RETURN -> {
          final long offset = Words.clampedToLong(hub.currentFrame().frame().getStackItem(0));
          final long size = Words.clampedToLong(hub.currentFrame().frame().getStackItem(1));
          memorySegmentToHash = hub.messageFrame().shadowReadMemory(offset, size);
        }
        case CREATE2 -> {
          final long offset = Words.clampedToLong(hub.currentFrame().frame().getStackItem(1));
          final long size = Words.clampedToLong(hub.currentFrame().frame().getStackItem(2));
          memorySegmentToHash = hub.messageFrame().shadowReadMemory(offset, size);
        }
        default ->
            throw new UnsupportedOperationException(
                "Hash was attempted by the following opcode: " + this.opCode().toString());
      }
      this.hashInfoKeccak = EWord.of(Hash.hash(memorySegmentToHash));
    }

    this.staticGas = gp.staticGas();

    if (opCode.isJump() && !Exceptions.stackException(exceptions)) {
      final BigInteger prospectivePcNew =
          hub.currentFrame().frame().getStackItem(0).toUnsignedBigInteger();
      final BigInteger codeSize = BigInteger.valueOf(hub.currentFrame().code().getSize());

      boolean noOutOfGasException = !Exceptions.outOfGasException(exceptions);
      boolean prospectivePcNewIsInBounds =
          codeSize.compareTo(prospectivePcNew) > 0 && noOutOfGasException;

      if (opCode.mnemonic().equals(OpCode.JUMPI)) {
        boolean nonzeroJumpCondition =
            !hub.currentFrame()
                .frame()
                .getStackItem(1)
                .toUnsignedBigInteger()
                .equals(BigInteger.ZERO);
        prospectivePcNewIsInBounds = prospectivePcNewIsInBounds && nonzeroJumpCondition;
      }

      jumpDestinationVettingRequired = prospectivePcNewIsInBounds;
    } else {
      jumpDestinationVettingRequired = false;
    }

    this.commonFragmentValues = commonFragmentValues;
    this.pushValue = opCode.isNonTrivialPush() ? EWord.of(getPushValue(hub)) : EWord.ZERO;
  }

  private Bytes getPushValue(Hub hub) {
    checkState(hub.opCodeData().isNonTrivialPush(), "getPushValue called for non-PUSH opcode");

    final int pc = hub.messageFrame().getPC();
    if (pc + 1 >= hub.messageFrame().getCode().getSize()) {
      return Bytes.EMPTY;
    }

    final Bytes byteCode = hub.messageFrame().getCode().getBytes();
    final int nBytesToPush = (opCode.byteValue() & 0xff) - EVM_INST_PUSH0;
    final int nLeftoverBytes = byteCode.size() - (pc + 1);
    final Bytes partialPushValue = byteCode.slice(pc + 1, min(nLeftoverBytes, nBytesToPush));
    return (nLeftoverBytes >= nBytesToPush)
        ? partialPushValue
        : rightPadTo(partialPushValue, nBytesToPush);
  }

  public static StackFragment prepare(
      final Hub hub,
      final Stack stack,
      final List<StackItem> stackItems,
      final short exceptions,
      final AbortingConditions aborts,
      final GasProjection gp,
      boolean isDeploying,
      CommonFragmentValues commonFragmentValues) {
    return new StackFragment(
        hub, stack, stackItems, exceptions, aborts, gp, isDeploying, commonFragmentValues);
  }

  private boolean traceLog() {
    return opCode.isLog() && !commonFragmentValues.callFrame().willRevert();
  }

  @Override
  public Trace.Hub trace(Trace.Hub trace) {

    final List<Function<Bytes, Trace.Hub>> valHiTracers =
        List.of(
            trace::pStackStackItemValueHi1,
            trace::pStackStackItemValueHi2,
            trace::pStackStackItemValueHi3,
            trace::pStackStackItemValueHi4);

    final List<Function<Bytes, Trace.Hub>> valLoTracers =
        List.of(
            trace::pStackStackItemValueLo1,
            trace::pStackStackItemValueLo2,
            trace::pStackStackItemValueLo3,
            trace::pStackStackItemValueLo4);

    final List<Function<Boolean, Trace.Hub>> popTracers =
        List.of(
            trace::pStackStackItemPop1,
            trace::pStackStackItemPop2,
            trace::pStackStackItemPop3,
            trace::pStackStackItemPop4);

    final List<Function<Short, Trace.Hub>> heightTracers =
        List.of(
            trace::pStackStackItemHeight1,
            trace::pStackStackItemHeight2,
            trace::pStackStackItemHeight3,
            trace::pStackStackItemHeight4);

    final List<Function<Integer, Trace.Hub>> stampTracers =
        List.of(
            trace::pStackStackItemStamp1,
            trace::pStackStackItemStamp2,
            trace::pStackStackItemStamp3,
            trace::pStackStackItemStamp4);

    var it = stackOps.listIterator();
    while (it.hasNext()) {
      var i = it.nextIndex();
      var op = it.next();
      final EWord eValue =
          (i == 3 && opCode().isNonTrivialPush()) ? pushValue : EWord.of(op.value());

      heightTracers.get(i).apply(op.height());
      valHiTracers.get(i).apply(eValue.hi());
      valLoTracers.get(i).apply(eValue.lo());
      popTracers.get(i).apply(op.action() == Stack.POP);
      stampTracers.get(i).apply(op.stackStamp());
    }

    final InstructionFamily currentInstFamily = stack.getCurrentOpcodeData().instructionFamily();
    final TracedException tracedException = commonFragmentValues.tracedException();

    this.tracedExceptionSanityChecks(tracedException);

    return trace
        .peekAtStack(true)
        // Instruction details
        .pStackAlpha(UnsignedByte.of(stack.getCurrentOpcodeData().stackSettings().alpha()))
        .pStackDelta(UnsignedByte.of(stack.getCurrentOpcodeData().stackSettings().delta()))
        .pStackInstruction(UnsignedByte.of(rawOpCode))
        .pStackStaticGas(staticGas)
        // Opcode families
        .pStackAccFlag(currentInstFamily == ACCOUNT)
        .pStackAddFlag(currentInstFamily == ADD)
        .pStackBinFlag(currentInstFamily == BIN)
        .pStackBtcFlag(currentInstFamily == BATCH)
        .pStackCallFlag(currentInstFamily == CALL)
        .pStackConFlag(currentInstFamily == CONTEXT)
        .pStackCopyFlag(currentInstFamily == COPY)
        .pStackCreateFlag(currentInstFamily == CREATE)
        .pStackDupFlag(currentInstFamily == DUP)
        .pStackExtFlag(currentInstFamily == EXT)
        .pStackHaltFlag(currentInstFamily == HALT)
        .pStackInvalidFlag(currentInstFamily == INVALID)
        .pStackJumpFlag(currentInstFamily == JUMP)
        .pStackKecFlag(currentInstFamily == KEC)
        .pStackLogFlag(currentInstFamily == LOG)
        .pStackMachineStateFlag(currentInstFamily == MACHINE_STATE)
        .pStackModFlag(currentInstFamily == MOD)
        .pStackMulFlag(currentInstFamily == MUL)
        .pStackPushpopFlag(currentInstFamily == PUSH_POP)
        .pStackShfFlag(currentInstFamily == SHF)
        .pStackStackramFlag(currentInstFamily == STACK_RAM)
        .pStackStoFlag(currentInstFamily == STORAGE)
        .pStackSwapFlag(currentInstFamily == SWAP)
        .pStackTxnFlag(currentInstFamily == TRANSACTION)
        .pStackWcpFlag(currentInstFamily == WCP)
        .pStackMcopyFlag(currentInstFamily == MCOPY)
        .pStackTransFlag(currentInstFamily == TRANSIENT)
        .pStackDecFlag1(stack.getCurrentOpcodeData().stackSettings().flag1())
        .pStackDecFlag2(stack.getCurrentOpcodeData().stackSettings().flag2())
        .pStackDecFlag3(stack.getCurrentOpcodeData().stackSettings().flag3())
        .pStackDecFlag4(stack.getCurrentOpcodeData().stackSettings().flag4())
        .pStackMxpFlag(stack.getCurrentOpcodeData().isMxp())
        .pStackStaticFlag(stack.getCurrentOpcodeData().stackSettings().forbiddenInStatic())
        .pStackPushValueHi(pushValue.hi())
        .pStackPushValueLo(pushValue.lo())
        .pStackJumpDestinationVettingRequired(jumpDestinationVettingRequired)
        // Exception flag
        .pStackOpcx(tracedException == INVALID_OPCODE)
        .pStackSux(tracedException == STACK_UNDERFLOW)
        .pStackSox(tracedException == STACK_OVERFLOW)
        .pStackMxpx(tracedException == MEMORY_EXPANSION_EXCEPTION)
        .pStackOogx(tracedException == OUT_OF_GAS_EXCEPTION)
        .pStackRdcx(tracedException == RETURN_DATA_COPY_FAULT)
        .pStackJumpx(tracedException == JUMP_FAULT)
        .pStackStaticx(tracedException == STATIC_FAULT)
        .pStackSstorex(tracedException == OUT_OF_SSTORE)
        .pStackIcpx(tracedException == INVALID_CODE_PREFIX)
        .pStackMaxcsx(tracedException == MAX_CODE_SIZE_EXCEPTION)
        // Hash data
        .pStackHashInfoFlag(hashInfoFlag)
        .pStackHashInfoKeccakHi(hashInfoKeccak.hi())
        .pStackHashInfoKeccakLo(hashInfoKeccak.lo())
        .pStackLogInfoFlag(traceLog());
  }

  private void tracedExceptionSanityChecks(TracedException tracedException) {
    switch (tracedException) {
      case NONE ->
          checkArgument(
              Exceptions.none(exceptions),
              "tracedException %s not present in exceptions %s",
              NONE,
              exceptions);
      case INVALID_OPCODE ->
          checkArgument(
              Exceptions.invalidOpcode(exceptions),
              "tracedException %s not present in exceptions %s",
              INVALID_OPCODE,
              exceptions);
      case STACK_UNDERFLOW ->
          checkArgument(
              Exceptions.stackUnderflow(exceptions),
              "tracedException %s not present in exceptions %s",
              STACK_UNDERFLOW,
              exceptions);
      case STACK_OVERFLOW ->
          checkArgument(
              Exceptions.stackOverflow(exceptions),
              "tracedException %s not present in exceptions %s",
              STACK_OVERFLOW,
              exceptions);
      case MEMORY_EXPANSION_EXCEPTION ->
          checkArgument(
              Exceptions.memoryExpansionException(exceptions),
              "tracedException %s not present in exceptions %s",
              MEMORY_EXPANSION_EXCEPTION,
              exceptions);
      case OUT_OF_GAS_EXCEPTION ->
          checkArgument(
              Exceptions.outOfGasException(exceptions),
              "tracedException %s not present in exceptions %s",
              OUT_OF_GAS_EXCEPTION,
              exceptions);
      case RETURN_DATA_COPY_FAULT ->
          checkArgument(
              Exceptions.returnDataCopyFault(exceptions),
              "tracedException %s not present in exceptions %s",
              RETURN_DATA_COPY_FAULT,
              exceptions);
      case JUMP_FAULT ->
          checkArgument(
              Exceptions.jumpFault(exceptions),
              "tracedException %s not present in exceptions %s",
              JUMP_FAULT,
              exceptions);
      case STATIC_FAULT ->
          checkArgument(
              Exceptions.staticFault(exceptions),
              "tracedException %s not present in exceptions %s",
              STATIC_FAULT,
              exceptions);
      case OUT_OF_SSTORE ->
          checkArgument(
              Exceptions.outOfSStore(exceptions),
              "tracedException %s not present in exceptions %s",
              OUT_OF_SSTORE,
              exceptions);
      case INVALID_CODE_PREFIX ->
          checkArgument(
              Exceptions.invalidCodePrefix(exceptions),
              "tracedException %s not present in exceptions %s",
              INVALID_CODE_PREFIX,
              exceptions);
      case MAX_CODE_SIZE_EXCEPTION ->
          checkArgument(
              Exceptions.maxCodeSizeException(exceptions),
              "tracedException %s not present in exceptions %s",
              MAX_CODE_SIZE_EXCEPTION,
              exceptions);
      case UNDEFINED ->
          throw new RuntimeException(
              "tracedException remained UNDEFINED but "
                  + Exceptions.prettyStringOf(this.opCode.mnemonic(), exceptions));
    }
  }
}
