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

package net.consensys.linea.zktracer.module.hub.fragment.imc.mmu;

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.zktracer.Trace.*;
import static net.consensys.linea.zktracer.Trace.Ecdata.*;
import static net.consensys.linea.zktracer.module.Util.rightPaddedSlice;
import static net.consensys.linea.zktracer.module.blake2fmodexpdata.BlakeModexpDataOperation.*;
import static net.consensys.linea.zktracer.module.hub.Hub.newIdentifierFromStamp;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_RIPEMD_160;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_SHA2_256;
import static net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata.BASE_MIN_OFFSET;
import static net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata.EBS_MIN_OFFSET;
import static net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata.MBS_MIN_OFFSET;
import static net.consensys.linea.zktracer.runtime.callstack.CallFrame.extractContiguousLimbsFromMemory;
import static net.consensys.linea.zktracer.types.Conversions.*;
import static net.consensys.linea.zktracer.types.Utils.leftPadTo;
import static org.hyperledger.besu.evm.internal.Words.clampedToLong;

import java.util.Optional;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.Setter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.blsdata.BlsDataOperation;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.defer.EndTransactionDefer;
import net.consensys.linea.zktracer.module.hub.fragment.TraceSubFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.opcode.*;
import net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment;
import net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata;
import net.consensys.linea.zktracer.module.hub.section.call.precompileSubsection.EllipticCurvePrecompileSubsection;
import net.consensys.linea.zktracer.module.hub.section.call.precompileSubsection.ModexpSubsection;
import net.consensys.linea.zktracer.module.hub.section.call.precompileSubsection.PrecompileSubsection;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import net.consensys.linea.zktracer.module.hub.state.State;
import net.consensys.linea.zktracer.runtime.LogData;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import net.consensys.linea.zktracer.types.EWord;
import net.consensys.linea.zktracer.types.MemoryRange;
import net.consensys.linea.zktracer.types.Range;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.evm.worldstate.WorldView;

/**
 * This class represents a call to the MMU. However, some MMU calls may have their actual content
 * only definitely defined at tracing time, post conflation. In these cases, subclasses of this
 * class implement this defer mechanism.
 */
@RequiredArgsConstructor
@Setter
@Getter
@Accessors(fluent = true)
public class MmuCall implements TraceSubFragment, EndTransactionDefer {
  protected boolean traceMe = true;

  protected int instruction = 0;
  protected int sourceId = 0;
  protected int targetId = 0;
  protected int auxId = 0;
  protected EWord sourceOffset = EWord.ZERO;
  protected EWord targetOffset = EWord.ZERO;
  protected long size = 0;
  protected long referenceOffset = 0;
  protected long referenceSize = 0;
  protected boolean successBit = false;
  protected Bytes limb1 = Bytes.EMPTY;
  protected Bytes limb2 = Bytes.EMPTY;
  protected long phase = 0;

  private Optional<Bytes> sourceRamBytes = Optional.empty();
  private Optional<Bytes> targetRamBytes = Optional.empty();
  private Optional<Bytes> exoBytes = Optional.empty();

  protected boolean exoIsRlpTxn = false;
  protected boolean exoIsLog = false;
  protected boolean exoIsRom = false;
  protected boolean exoIsKec = false;
  protected boolean exoIsRipSha = false;
  protected boolean exoIsBlakeModexp = false;
  protected boolean exoIsEcData = false;
  protected boolean exoIsBlsData = false;
  private int exoSum = 0;

  public void dontTraceMe() {
    traceMe = false;
  }

  private MmuCall updateExoSum(final int exoValue) {
    exoSum += exoValue;
    return this;
  }

  final MmuCall setRlpTxn() {
    return this.exoIsRlpTxn(true).updateExoSum(EXO_SUM_WEIGHT_TXCD);
  }

  public final MmuCall setLog() {
    return this.exoIsLog(true).updateExoSum(EXO_SUM_WEIGHT_LOG);
  }

  public final void setRom() {
    this.exoIsRom(true).updateExoSum(EXO_SUM_WEIGHT_ROM);
  }

  public final MmuCall setKec() {
    return this.exoIsKec(true).updateExoSum(EXO_SUM_WEIGHT_KEC);
  }

  final MmuCall setRipSha() {
    return this.exoIsRipSha(true).updateExoSum(EXO_SUM_WEIGHT_RIPSHA);
  }

  final MmuCall setBlakeModexp() {
    return this.exoIsBlakeModexp(true).updateExoSum(EXO_SUM_WEIGHT_BLAKEMODEXP);
  }

  final MmuCall setBlakeModexp(boolean effectiveFlag) {
    return this.exoIsBlakeModexp(effectiveFlag).updateExoSum(EXO_SUM_WEIGHT_BLAKEMODEXP);
  }

  final MmuCall setEcData() {
    return this.exoIsEcData(true).updateExoSum(EXO_SUM_WEIGHT_ECDATA);
  }

  final MmuCall setBlsData() {
    return this.exoIsBlsData(true).updateExoSum(EXO_SUM_WEIGHT_BLSDATA);
  }

  final MmuCall setEllipticCurveModule(PrecompileScenarioFragment.PrecompileFlag flag) {
    return flag.isEcdataPrecompile() ? setEcData() : setBlsData();
  }

  public MmuCall(final Hub hub, final int instruction) {
    hub.defers().scheduleForEndTransaction(this);
    this.instruction = instruction;
  }

  public static MmuCall sha3(final Hub hub, final Bytes hashInput) {
    final CallFrame currentFrame = hub.currentFrame();
    final Bytes sourceOffset = currentFrame.frame().getStackItem(0);
    final Bytes size = currentFrame.frame().getStackItem(1);
    return new MmuCall(hub, MMU_INST_RAM_TO_EXO_WITH_PADDING)
        .sourceId(currentFrame.contextNumber())
        .sourceRamBytes(
            Optional.of(
                extractContiguousLimbsFromMemory(
                    currentFrame.frame(), Range.fromOffsetAndSize(sourceOffset, size))))
        .auxId(newIdentifierFromStamp(hub.stamp()))
        .exoBytes(Optional.of(hashInput))
        .sourceOffset(EWord.of(sourceOffset))
        .size(clampedToLong(size))
        .referenceSize(clampedToLong(size))
        .setKec();
  }

  public static MmuCall callDataCopy(final Hub hub) {
    final CallFrame currentFrame = hub.currentFrame();
    final MemoryRange callDataRange = currentFrame.callDataRange();
    final Bytes sourceBytes = hub.callStack().getFullMemoryOfCaller(hub);

    return new MmuCall(hub, MMU_INST_ANY_TO_RAM_WITH_PADDING)
        .sourceId((int) callDataRange.contextNumber())
        .sourceRamBytes(Optional.of(sourceBytes))
        .targetId(currentFrame.contextNumber())
        .targetRamBytes(
            Optional.of(
                currentFrame.frame().shadowReadMemory(0, currentFrame.frame().memoryByteSize())))
        .sourceOffset(EWord.of(currentFrame.frame().getStackItem(1)))
        .targetOffset(EWord.of(currentFrame.frame().getStackItem(0)))
        .size(clampedToLong(currentFrame.frame().getStackItem(2)))
        .referenceOffset(callDataRange.offset())
        .referenceSize(callDataRange.size());
  }

  public static MmuCall callDataLoad(final Hub hub) {
    return new CallDataLoad(hub);
  }

  public static MmuCall LogX(final Hub hub, final LogData logData) {
    return new MmuCall(hub, MMU_INST_RAM_TO_EXO_WITH_PADDING)
        .sourceId(logData.callFrame.contextNumber())
        .sourceRamBytes(Optional.of(logData.ramSourceBytes))
        .exoBytes(
            Optional.of(
                rightPaddedSlice(
                    logData.ramSourceBytes,
                    (int) clampedToLong(logData.offset),
                    (int) logData.size)))
        .sourceOffset(logData.offset)
        .size(logData.size)
        .referenceSize(logData.size)
        .setLog();

    // Note: The targetId() is set at the end of the transaction. We don't know now if the LOG will
    // be reverted or not

  }

  public static MmuCall codeCopy(final Hub hub) {
    return new CodeCopy(hub);
  }

  public static MmuCall extCodeCopy(final Hub hub) {
    return new ExtCodeCopy(hub);
  }

  public static MmuCall returnDataCopy(final Hub hub) {
    final CallFrame currentFrame = hub.currentFrame();
    final MemoryRange returnDataRange = currentFrame.returnDataRange();
    final CallFrame returnerFrame =
        hub.callStack().getByContextNumber(returnDataRange.contextNumber());

    return new MmuCall(hub, MMU_INST_ANY_TO_RAM_WITH_PADDING)
        .sourceId(returnerFrame.contextNumber())
        .sourceRamBytes(Optional.of(returnDataRange.getRawData()))
        .targetId(currentFrame.contextNumber())
        .targetRamBytes(
            Optional.of(
                currentFrame.frame().shadowReadMemory(0, currentFrame.frame().memoryByteSize())))
        .sourceOffset(EWord.of(currentFrame.frame().getStackItem(1)))
        .targetOffset(EWord.of(currentFrame.frame().getStackItem(0)))
        .size(clampedToLong(currentFrame.frame().getStackItem(2)))
        .referenceOffset(returnDataRange.offset())
        .referenceSize(returnDataRange.size());
  }

  public static MmuCall create(final Hub hub) {
    return new Create(hub);
  }

  public static ReturnFromDeploymentMmuCall returnFromDeployment(final Hub hub) {
    return new ReturnFromDeploymentMmuCall(hub);
  }

  public static MmuCall returnFromMessageCall(final Hub hub) {
    return MmuCall.revert(hub);
  }

  public static MmuCall create2(
      final Hub hub, final Bytes create2initCode, final boolean failureCondition) {
    return new Create2(hub, create2initCode, failureCondition);
  }

  public static MmuCall invalidCodePrefix(final Hub hub) {
    final short currentExceptions = hub.pch().exceptions();

    return new MmuCall(hub, MMU_INST_INVALID_CODE_PREFIX)
        .sourceId(hub.currentFrame().contextNumber())
        .sourceRamBytes(
            Optional.of(
                hub.currentFrame()
                    .frame()
                    .shadowReadMemory(0, hub.currentFrame().frame().memoryByteSize())))
        .sourceOffset(EWord.of(hub.messageFrame().getStackItem(0)))
        .successBit(Exceptions.invalidCodePrefix(currentExceptions));
  }

  public static MmuCall revert(final Hub hub) {
    final CallFrame parentFrame = hub.callStack().parentCallFrame();

    return new MmuCall(hub, MMU_INST_RAM_TO_RAM_SANS_PADDING)
        .sourceId(hub.currentFrame().contextNumber())
        .sourceRamBytes(
            Optional.of(
                hub.currentFrame()
                    .frame()
                    .shadowReadMemory(0, hub.currentFrame().frame().memoryByteSize())))
        .targetId(parentFrame.contextNumber())
        .targetRamBytes(
            Optional.of(
                parentFrame
                    .frame()
                    .shadowReadMemory(
                        0, hub.callStack().parentCallFrame().frame().memoryByteSize())))
        .sourceOffset(EWord.of(hub.messageFrame().getStackItem(0)))
        .size(clampedToLong(hub.messageFrame().getStackItem(1)))
        .referenceOffset(hub.currentFrame().returnAtRange().offset())
        .referenceSize(hub.currentFrame().returnAtRange().size());
  }

  public static MmuCall txInit(final Hub hub) {
    return new MmuCall(hub, MMU_INST_EXO_TO_RAM_TRANSPLANTS)
        .sourceId(hub.txStack().current().getUserTransactionNumber())
        .exoBytes(Optional.of(hub.txStack().current().getBesuTransaction().getPayload()))
        .targetId(hub.stamp())
        .targetRamBytes(Optional.of(Bytes.EMPTY))
        .size(hub.txStack().current().getBesuTransaction().getData().map(Bytes::size).orElse(0))
        .phase(RLP_TXN_PHASE_DATA)
        .setRlpTxn();
  }

  public static MmuCall callDataExtractionForEcrecover(
      final Hub hub, EllipticCurvePrecompileSubsection subsection, boolean successfulRecovery) {

    return new MmuCall(hub, MMU_INST_RAM_TO_EXO_WITH_PADDING)
        .sourceId(hub.currentFrame().contextNumber()) // called at ContextReEntry
        .sourceRamBytes(Optional.of(subsection.rawCallerMemory()))
        .targetId(subsection.exoModuleOperationId())
        .exoBytes(Optional.of(subsection.extractCallData()))
        .sourceOffset(EWord.of(subsection.callDataOffset()))
        .size(subsection.callDataSize())
        .referenceSize(TOTAL_SIZE_ECRECOVER_DATA)
        .successBit(successfulRecovery)
        .phase(PHASE_ECRECOVER_DATA)
        .setEcData();
  }

  public static MmuCall fullReturnDataTransferForEcrecover(
      final Hub hub, EllipticCurvePrecompileSubsection subsection, boolean successBit) {

    final int precompileContextNumber = subsection.exoModuleOperationId();

    checkState(
        subsection.returnDataRange.getRange().size() == TOTAL_SIZE_ECRECOVER_RESULT,
        String.format(
            "MmuCall: Ecrecover return data size is %d but is expected to be %d",
            subsection.returnDataRange.getRange().size(), TOTAL_SIZE_ECRECOVER_RESULT));

    return new MmuCall(hub, MMU_INST_EXO_TO_RAM_TRANSPLANTS)
        .sourceId(precompileContextNumber)
        .exoBytes(Optional.of(subsection.returnDataRange.extract()))
        .targetId(precompileContextNumber)
        .targetRamBytes(Optional.of(Bytes.EMPTY))
        .size(TOTAL_SIZE_ECRECOVER_RESULT)
        .phase(PHASE_ECRECOVER_RESULT)
        .successBit(successBit)
        .setEcData();
  }

  public static MmuCall partialReturnDataCopyForEcrecover(
      final Hub hub, EllipticCurvePrecompileSubsection subsection) {

    final int precompileContextNumber = subsection.exoModuleOperationId();

    return new MmuCall(hub, MMU_INST_RAM_TO_RAM_SANS_PADDING)
        .sourceId(precompileContextNumber)
        .sourceRamBytes(Optional.of(subsection.returnDataRange.extract()))
        .targetId(hub.currentFrame().contextNumber())
        .targetRamBytes(Optional.of(subsection.rawCallerMemory()))
        .sourceOffset(EWord.ZERO)
        .size(TOTAL_SIZE_ECRECOVER_RESULT)
        .referenceOffset(subsection.returnAtOffset())
        .referenceSize(subsection.returnAtCapacity());
  }

  public static MmuCall callDataExtractionForShaTwoAndRipemd(
      final Hub hub, PrecompileSubsection subsection) {

    final PrecompileScenarioFragment.PrecompileFlag flag =
        subsection.precompileScenarioFragment().flag;
    checkArgument(
        flag.isAnyOf(PRC_SHA2_256, PRC_RIPEMD_160),
        "Unexpected precompile %s, only accept %s and %s",
        flag,
        PRC_SHA2_256,
        PRC_RIPEMD_160);

    return new MmuCall(hub, MMU_INST_RAM_TO_EXO_WITH_PADDING)
        .sourceId(hub.currentFrame().contextNumber())
        .sourceRamBytes(Optional.of(subsection.rawCallerMemory()))
        .targetId(subsection.exoModuleOperationId())
        .exoBytes(Optional.of(subsection.extractCallData()))
        .sourceOffset(EWord.of(subsection.callDataOffset()))
        .size(subsection.callDataSize())
        .referenceSize(subsection.callDataSize())
        .phase(flag.dataPhase())
        .setRipSha();
  }

  public static MmuCall fullResultTransferForShaTwoAndRipemd(
      final Hub hub, PrecompileSubsection subsection) {

    final PrecompileScenarioFragment.PrecompileFlag flag =
        subsection.precompileScenarioFragment().flag;
    checkArgument(
        flag.isAnyOf(PRC_SHA2_256, PRC_RIPEMD_160),
        "Unexpected precompile %s, only accept %s and %s",
        flag,
        PRC_SHA2_256,
        PRC_RIPEMD_160);

    final boolean isShaTwo = flag == PRC_SHA2_256;

    if (subsection.getCallDataRange().isEmpty()) {
      return new MmuCall(hub, MMU_INST_MSTORE)
          .targetId(subsection.exoModuleOperationId())
          .targetOffset(EWord.ZERO)
          .limb1(isShaTwo ? bigIntegerToBytes(EMPTY_SHA2_HI) : longToBytes(EMPTY_RIPEMD_HI))
          .limb2(isShaTwo ? bigIntegerToBytes(EMPTY_SHA2_LO) : bigIntegerToBytes(EMPTY_RIPEMD_LO));
    } else {
      return new MmuCall(hub, MMU_INST_EXO_TO_RAM_TRANSPLANTS)
          .sourceId(subsection.exoModuleOperationId())
          .exoBytes(Optional.of(leftPadTo(subsection.returnDataRange.extract(), WORD_SIZE)))
          .targetId(subsection.returnDataContextNumber())
          .targetRamBytes(Optional.of(Bytes.EMPTY))
          .size(WORD_SIZE)
          .phase(flag.resultPhase())
          .setRipSha();
    }
  }

  public static MmuCall partialCopyOfReturnDataForShaTwoAndRipemd(
      final Hub hub, PrecompileSubsection subsection) {

    final PrecompileScenarioFragment.PrecompileFlag flag =
        subsection.precompileScenarioFragment().flag;

    checkArgument(
        flag.isAnyOf(PRC_SHA2_256, PRC_RIPEMD_160),
        "Unexpected precompile %s, only accept %s and %s",
        flag,
        PRC_SHA2_256,
        PRC_RIPEMD_160);
    checkArgument(
        !subsection.getReturnAtRange().isEmpty(),
        "Partial copy of return data cannot be done if the `returnAtRange` is empty");

    return new MmuCall(hub, MMU_INST_RAM_TO_RAM_SANS_PADDING)
        .sourceId(subsection.returnDataContextNumber())
        .sourceRamBytes(Optional.of(leftPadTo(subsection.returnDataRange.extract(), WORD_SIZE)))
        .targetId(hub.currentFrame().contextNumber())
        .targetRamBytes(Optional.of(subsection.rawCallerMemory()))
        .sourceOffset(EWord.ZERO)
        .size(WORD_SIZE)
        .referenceOffset(subsection.returnAtOffset())
        .referenceSize(subsection.returnAtCapacity());
  }

  public static MmuCall callDataExtractionForIdentity(
      final Hub hub, PrecompileSubsection subsection) {

    return new MmuCall(hub, MMU_INST_RAM_TO_RAM_SANS_PADDING)
        .sourceId(hub.currentFrame().contextNumber()) // called at ContextReEntry
        .sourceRamBytes(Optional.of(subsection.rawCallerMemory()))
        .targetId(subsection.exoModuleOperationId())
        .targetRamBytes(Optional.of(Bytes.EMPTY))
        .sourceOffset(EWord.of(subsection.callDataOffset()))
        .size(subsection.callDataSize())
        .referenceSize(subsection.callDataSize());
  }

  public static MmuCall partialCopyOfReturnDataForIdentity(
      final Hub hub, final PrecompileSubsection subsection) {

    checkState(
        subsection.callDataSize() == subsection.returnDataSize(),
        "MmuCall: the IDENTITY precompile should have <call data size> == <return data size>");
    checkState(
        subsection.returnDataOffset() == 0,
        "MmuCall: the IDENTITY precompile store its <return data> starting at offset 0");

    return new MmuCall(hub, MMU_INST_RAM_TO_RAM_SANS_PADDING)
        .sourceId(subsection.exoModuleOperationId())
        .sourceRamBytes(Optional.of(subsection.returnDataRange.extract()))
        .targetId(hub.currentFrame().contextNumber())
        .targetRamBytes(Optional.of(subsection.rawCallerMemory()))
        .sourceOffset(EWord.ZERO)
        .size(subsection.callDataSize())
        .referenceOffset(subsection.returnAtOffset())
        .referenceSize(subsection.returnAtCapacity());
  }

  public static MmuCall callDataExtractionForEcadd(
      final Hub hub, PrecompileSubsection subsection, boolean successBit) {
    return new MmuCall(hub, MMU_INST_RAM_TO_EXO_WITH_PADDING)
        .sourceId(hub.currentFrame().contextNumber())
        .sourceRamBytes(Optional.of(subsection.rawCallerMemory()))
        .targetId(subsection.exoModuleOperationId())
        .exoBytes(Optional.of(subsection.extractCallData()))
        .sourceOffset(EWord.of(subsection.callDataOffset()))
        .size(subsection.callDataSize())
        .referenceSize(TOTAL_SIZE_ECADD_DATA)
        .successBit(successBit)
        .setEcData()
        .phase(PHASE_ECADD_DATA);
  }

  public static MmuCall fullTransferOfReturnDataForEcadd(
      final Hub hub, PrecompileSubsection subsection, boolean successBit) {
    return new MmuCall(hub, MMU_INST_EXO_TO_RAM_TRANSPLANTS)
        .sourceId(subsection.exoModuleOperationId())
        .exoBytes(
            Optional.of(leftPadTo(subsection.returnDataRange.extract(), TOTAL_SIZE_ECADD_RESULT)))
        .targetId(subsection.exoModuleOperationId())
        .targetRamBytes(Optional.of(Bytes.EMPTY))
        .size(TOTAL_SIZE_ECADD_RESULT)
        .setEcData()
        .phase(PHASE_ECADD_RESULT)
        .successBit(successBit);
  }

  public static MmuCall partialCopyOfReturnDataForEcadd(
      final Hub hub, PrecompileSubsection subsection) {
    return new MmuCall(hub, MMU_INST_RAM_TO_RAM_SANS_PADDING)
        .sourceId(subsection.exoModuleOperationId())
        .sourceRamBytes(
            Optional.of(leftPadTo(subsection.returnDataRange.extract(), TOTAL_SIZE_ECADD_RESULT)))
        .targetId(hub.currentFrame().contextNumber())
        .targetRamBytes(Optional.of(subsection.rawCallerMemory()))
        .size(TOTAL_SIZE_ECADD_RESULT)
        .referenceOffset(subsection.returnAtOffset())
        .referenceSize(subsection.returnAtCapacity());
  }

  public static MmuCall callDataExtractionForEcmul(
      final Hub hub, final PrecompileSubsection subsection, boolean successBit) {
    return new MmuCall(hub, MMU_INST_RAM_TO_EXO_WITH_PADDING)
        .sourceId(hub.currentFrame().contextNumber())
        .sourceRamBytes(Optional.of(subsection.rawCallerMemory()))
        .targetId(subsection.exoModuleOperationId())
        .exoBytes(Optional.of(subsection.extractCallData()))
        .sourceOffset(EWord.of(subsection.callDataOffset()))
        .size(subsection.callDataSize())
        .referenceSize(TOTAL_SIZE_ECMUL_DATA)
        .successBit(successBit)
        .setEcData()
        .phase(PHASE_ECMUL_DATA);
  }

  public static MmuCall fullReturnDataTransferForEcmul(
      final Hub hub, final PrecompileSubsection subsection, boolean successBit) {
    return new MmuCall(hub, MMU_INST_EXO_TO_RAM_TRANSPLANTS)
        .sourceId(subsection.exoModuleOperationId())
        .exoBytes(
            Optional.of(leftPadTo(subsection.returnDataRange.extract(), TOTAL_SIZE_ECMUL_RESULT)))
        .targetId(subsection.exoModuleOperationId())
        .targetRamBytes(Optional.of(Bytes.EMPTY))
        .size(TOTAL_SIZE_ECMUL_RESULT)
        .setEcData()
        .phase(PHASE_ECMUL_RESULT)
        .successBit(successBit);
  }

  public static MmuCall partialCopyOfReturnDataForEcmul(
      final Hub hub, final PrecompileSubsection subsection) {
    return new MmuCall(hub, MMU_INST_RAM_TO_RAM_SANS_PADDING)
        .sourceId(subsection.exoModuleOperationId())
        .sourceRamBytes(
            Optional.of(leftPadTo(subsection.returnDataRange.extract(), TOTAL_SIZE_ECMUL_RESULT)))
        .targetId(hub.currentFrame().contextNumber())
        .targetRamBytes(Optional.of(subsection.rawCallerMemory()))
        .size(TOTAL_SIZE_ECMUL_RESULT)
        .referenceOffset(subsection.returnAtOffset())
        .referenceSize(subsection.returnAtCapacity());
  }

  public static MmuCall callDataExtractionForEcpairing(
      final Hub hub, PrecompileSubsection subsection, boolean successBit) {
    final int precompileContextNumber = subsection.exoModuleOperationId();
    return new MmuCall(hub, MMU_INST_RAM_TO_EXO_WITH_PADDING)
        .sourceId(hub.currentFrame().contextNumber())
        .sourceRamBytes(Optional.of(subsection.rawCallerMemory()))
        .targetId(precompileContextNumber)
        .exoBytes(Optional.of(subsection.extractCallData()))
        .sourceOffset(EWord.of(subsection.callDataOffset()))
        .size(subsection.callDataSize())
        .referenceSize(subsection.callDataSize())
        .successBit(successBit)
        .setEcData()
        .phase(PHASE_ECPAIRING_DATA);
  }

  public static MmuCall callDataExtractionForPostCancunPrecompiles(
      Hub hub, EllipticCurvePrecompileSubsection subsection, boolean successBit) {
    final int precompileContextNumber = subsection.exoModuleOperationId();
    return new MmuCall(hub, MMU_INST_RAM_TO_EXO_WITH_PADDING) // Note: there will be no padding
        .sourceId(hub.currentFrame().contextNumber())
        .sourceRamBytes(Optional.of(subsection.rawCallerMemory()))
        .targetId(precompileContextNumber)
        .exoBytes(Optional.of(subsection.extractCallData()))
        .sourceOffset(EWord.of(subsection.callDataOffset()))
        .size(subsection.callDataSize())
        .referenceSize(subsection.callDataSize())
        // constant
        .successBit(successBit)
        .phase(subsection.flag().dataPhase())
        .setEllipticCurveModule(subsection.flag());
  }

  public static MmuCall fullReturnDataTransferForPostCancunPrecompiles(
      final Hub hub, EllipticCurvePrecompileSubsection subsection, boolean successBit) {
    final int precompileContextNumber = subsection.exoModuleOperationId();

    final long expectedReturnDataSize =
        subsection.flag() != PrecompileScenarioFragment.PrecompileFlag.PRC_P256_VERIFY
            ? BlsDataOperation.expectedReturnDataSize(subsection.flag())
            : PRECOMPILE_RETURN_DATA_SIZE___P256_VERIFY;
    checkState(
        subsection.returnDataRange.getRange().size() == expectedReturnDataSize,
        "The return data size for post-cancun precompile does not match our expectation of it");

    return new MmuCall(hub, MMU_INST_EXO_TO_RAM_TRANSPLANTS)
        .sourceId(precompileContextNumber)
        .exoBytes(Optional.of(subsection.returnDataRange.extract()))
        .targetId(precompileContextNumber)
        .targetRamBytes(Optional.of(Bytes.EMPTY))
        .size(expectedReturnDataSize)
        .phase(subsection.flag().resultPhase())
        .successBit(successBit)
        .setEllipticCurveModule(subsection.flag());
  }

  public static MmuCall partialCopyOfReturnDataForPostCancunPrecompiles(
      final Hub hub, PrecompileSubsection subsection) {
    final int precompileContextNumber = subsection.exoModuleOperationId();
    final int returnDataSize = (int) subsection.returnDataRange.getRange().size();

    return new MmuCall(hub, MMU_INST_RAM_TO_RAM_SANS_PADDING)
        .sourceId(precompileContextNumber)
        .sourceRamBytes(Optional.of(subsection.returnDataRange.extract()))
        .targetId(hub.currentFrame().contextNumber())
        .targetRamBytes(Optional.of(subsection.rawCallerMemory()))
        .size(returnDataSize)
        .referenceOffset(subsection.returnAtOffset())
        .referenceSize(subsection.returnAtCapacity())
        .setEllipticCurveModule(subsection.flag());
  }

  /**
   * Note that {@link MmuCall#fullReturnDataTransferForEcpairing} handles both cases of interest:
   * empty call data and nonempty call data.
   */
  public static MmuCall fullReturnDataTransferForEcpairing(
      final Hub hub, PrecompileSubsection subsection, boolean successBit) {
    final int precompileContextNumber = subsection.exoModuleOperationId();
    if (subsection.getCallDataRange().isEmpty()) {
      return new MmuCall(hub, MMU_INST_MSTORE).targetId(precompileContextNumber).limb2(Bytes.of(1));
    } else {
      return new MmuCall(hub, MMU_INST_EXO_TO_RAM_TRANSPLANTS)
          .sourceId(precompileContextNumber)
          .exoBytes(
              Optional.of(
                  leftPadTo(subsection.returnDataRange.extract(), TOTAL_SIZE_ECPAIRING_RESULT)))
          .targetId(precompileContextNumber)
          .targetRamBytes(Optional.of(Bytes.EMPTY))
          .size(TOTAL_SIZE_ECPAIRING_RESULT)
          .setEcData()
          .phase(PHASE_ECPAIRING_RESULT)
          .successBit(successBit);
    }
  }

  public static MmuCall partialCopyOfReturnDataForEcpairing(
      final Hub hub, PrecompileSubsection subsection) {
    final int precompileContextNumber = subsection.exoModuleOperationId();
    return new MmuCall(hub, MMU_INST_RAM_TO_RAM_SANS_PADDING)
        .sourceId(precompileContextNumber)
        .sourceRamBytes(
            Optional.of(
                leftPadTo(subsection.returnDataRange.extract(), TOTAL_SIZE_ECPAIRING_RESULT)))
        .targetId(hub.currentFrame().contextNumber())
        .targetRamBytes(Optional.of(subsection.rawCallerMemory()))
        .size(TOTAL_SIZE_ECPAIRING_RESULT)
        .referenceOffset(subsection.returnAtOffset())
        .referenceSize(subsection.returnAtCapacity());
  }

  public static MmuCall parameterExtractionForBlake(
      final Hub hub,
      PrecompileSubsection subsection,
      boolean blakeSuccess,
      Bytes blakeR,
      Bytes blakeF) {
    final int precompileContextNumber = subsection.exoModuleOperationId();
    return new MmuCall(hub, MMU_INST_BLAKE)
        .sourceId(hub.currentFrame().contextNumber())
        .sourceRamBytes(Optional.of(subsection.rawCallerMemory()))
        .targetId(precompileContextNumber)
        .sourceOffset(EWord.of(subsection.callDataOffset()))
        .successBit(blakeSuccess)
        .limb1(blakeR)
        .limb2(blakeF)
        .setBlakeModexp(blakeSuccess)
        .phase(PHASE_BLAKE_PARAMS);
  }

  public static MmuCall callDataExtractionforBlake(final Hub hub, PrecompileSubsection subsection) {
    final int precompileContextNumber = subsection.exoModuleOperationId();
    return new MmuCall(hub, MMU_INST_RAM_TO_EXO_WITH_PADDING)
        .sourceId(hub.currentFrame().contextNumber())
        .sourceRamBytes(Optional.of(subsection.rawCallerMemory()))
        .targetId(precompileContextNumber)
        .exoBytes(
            Optional.of(
                subsection
                    .extractCallData()
                    .slice(BLAKE2f_HASH_INPUT_OFFSET, BLAKE2f_HASH_INPUT_SIZE)))
        .sourceOffset(EWord.of(subsection.callDataOffset() + BLAKE2f_HASH_INPUT_OFFSET))
        .size(BLAKE2f_HASH_INPUT_SIZE)
        .referenceSize(BLAKE2f_HASH_INPUT_SIZE)
        .setBlakeModexp()
        .phase(PHASE_BLAKE_DATA);
  }

  public static MmuCall fullReturnDataTransferForBlake(
      final Hub hub, PrecompileSubsection subsection) {
    final int precompileContextNumber = subsection.exoModuleOperationId();
    return new MmuCall(hub, MMU_INST_EXO_TO_RAM_TRANSPLANTS)
        .sourceId(precompileContextNumber)
        .exoBytes(
            Optional.of(leftPadTo(subsection.returnDataRange.extract(), BLAKE2f_HASH_OUTPUT_SIZE)))
        .targetId(precompileContextNumber)
        .targetRamBytes(Optional.of(Bytes.EMPTY))
        .size(BLAKE2f_HASH_OUTPUT_SIZE)
        .setBlakeModexp()
        .phase(PHASE_BLAKE_RESULT);
  }

  public static MmuCall partialCopyOfReturnDataforBlake(
      final Hub hub, PrecompileSubsection subsection) {
    final int precompileContextNumber = subsection.exoModuleOperationId();
    return new MmuCall(hub, MMU_INST_RAM_TO_RAM_SANS_PADDING)
        .sourceId(precompileContextNumber)
        .sourceRamBytes(
            Optional.of(leftPadTo(subsection.returnDataRange.extract(), BLAKE2f_HASH_OUTPUT_SIZE)))
        .targetId(hub.currentFrame().contextNumber())
        .targetRamBytes(Optional.of(subsection.rawCallerMemory()))
        .size(BLAKE2f_HASH_OUTPUT_SIZE)
        .referenceOffset(subsection.returnAtOffset())
        .referenceSize(subsection.returnAtCapacity());
  }

  public static MmuCall extractBbsForModexp(
      final Hub hub, final ModexpSubsection subsection, final ModexpMetadata metaData) {
    return new MmuCall(hub, MMU_INST_RIGHT_PADDED_WORD_EXTRACTION)
        .sourceId(hub.currentFrame().contextNumber())
        .sourceRamBytes(Optional.of(subsection.rawCallerMemory()))
        .referenceOffset(subsection.callDataOffset())
        .referenceSize(subsection.callDataSize())
        .limb1(metaData.bbs().hi())
        .limb2(metaData.bbs().lo());
  }

  public static MmuCall extractEbsForModexp(
      final Hub hub, final ModexpSubsection subsection, final ModexpMetadata metaData) {
    return new MmuCall(hub, MMU_INST_RIGHT_PADDED_WORD_EXTRACTION)
        .sourceId(hub.currentFrame().contextNumber())
        .sourceRamBytes(Optional.of(subsection.rawCallerMemory()))
        .sourceOffset(EWord.of(EBS_MIN_OFFSET))
        .referenceOffset(subsection.callDataOffset())
        .referenceSize(subsection.callDataSize())
        .limb1(metaData.ebs().hi())
        .limb2(metaData.ebs().lo());
  }

  public static MmuCall extractMbsForModexp(
      final Hub hub, final ModexpSubsection subsection, final ModexpMetadata metaData) {
    return new MmuCall(hub, MMU_INST_RIGHT_PADDED_WORD_EXTRACTION)
        .sourceId(hub.currentFrame().contextNumber())
        .sourceRamBytes(Optional.of(subsection.rawCallerMemory()))
        .sourceOffset(EWord.of(MBS_MIN_OFFSET))
        .referenceOffset(subsection.callDataOffset())
        .referenceSize(subsection.callDataSize())
        .limb1(metaData.mbs().hi())
        .limb2(metaData.mbs().lo());
  }

  public static MmuCall forModexpLoadLead(
      final Hub hub, final ModexpSubsection subsection, final ModexpMetadata metaData) {
    return new MmuCall(hub, MMU_INST_MLOAD)
        .sourceId(hub.currentFrame().contextNumber())
        .sourceRamBytes(Optional.of(subsection.rawCallerMemory()))
        .sourceOffset(EWord.of(subsection.callDataOffset() + BASE_MIN_OFFSET + metaData.bbsInt()))
        .limb1(metaData.rawLeadingWord().hi())
        .limb2(metaData.rawLeadingWord().lo());
  }

  public static MmuCall extractModexpBase(
      final Hub hub, final ModexpSubsection subsection, final ModexpMetadata modExpMetadata) {
    if (modExpMetadata.extractBase()) {
      return new MmuCall(hub, MMU_INST_MODEXP_DATA)
          .sourceId(hub.currentFrame().contextNumber()) // called at ContextReEntry
          .sourceRamBytes(Optional.of(subsection.rawCallerMemory()))
          .targetId(subsection.exoModuleOperationId())
          .exoBytes(Optional.of(leftPadTo(modExpMetadata.base(), modExpMetadata.getMaxInputSize())))
          .sourceOffset(EWord.of(BASE_MIN_OFFSET))
          .size(modExpMetadata.bbs().toInt())
          .referenceOffset(subsection.callDataOffset())
          .referenceSize(subsection.callDataSize())
          .phase(PHASE_MODEXP_BASE)
          .setBlakeModexp();
    } else {
      return new MmuCall(hub, MMU_INST_MODEXP_ZERO)
          .targetId(subsection.exoModuleOperationId())
          .phase(PHASE_MODEXP_BASE)
          .setBlakeModexp();
    }
  }

  public static MmuCall extractModexpExponent(
      final Hub hub, final ModexpSubsection subsection, final ModexpMetadata modExpMetadata) {
    if (modExpMetadata.extractExponent()) {
      return new MmuCall(hub, MMU_INST_MODEXP_DATA)
          .sourceId(hub.currentFrame().contextNumber()) // called at ContextReEntry
          .sourceRamBytes(Optional.of(subsection.rawCallerMemory()))
          .targetId(subsection.exoModuleOperationId())
          .exoBytes(Optional.of(leftPadTo(modExpMetadata.exp(), modExpMetadata.getMaxInputSize())))
          .sourceOffset(EWord.of(BASE_MIN_OFFSET + modExpMetadata.bbs().toInt()))
          .size(modExpMetadata.ebs().toInt())
          .referenceOffset(subsection.callDataOffset())
          .referenceSize(subsection.callDataSize())
          .phase(PHASE_MODEXP_EXPONENT)
          .setBlakeModexp();
    } else {
      return new MmuCall(hub, MMU_INST_MODEXP_ZERO)
          .targetId(subsection.exoModuleOperationId())
          .phase(PHASE_MODEXP_EXPONENT)
          .setBlakeModexp();
    }
  }

  public static MmuCall extractModexpModulus(
      final Hub hub, final ModexpSubsection subsection, final ModexpMetadata modExpMetadata) {
    return new MmuCall(hub, MMU_INST_MODEXP_DATA)
        .sourceId(hub.currentFrame().contextNumber()) // called at ContextReEntry
        .sourceRamBytes(Optional.of(subsection.rawCallerMemory()))
        .targetId(subsection.exoModuleOperationId())
        .exoBytes(Optional.of(leftPadTo(modExpMetadata.mod(), modExpMetadata.getMaxInputSize())))
        .sourceOffset(
            EWord.of(BASE_MIN_OFFSET + modExpMetadata.bbs().toInt() + modExpMetadata.ebs().toInt()))
        .size(modExpMetadata.mbs().toInt())
        .referenceOffset(subsection.callDataOffset())
        .referenceSize(subsection.callDataSize())
        .phase(PHASE_MODEXP_MODULUS)
        .setBlakeModexp();
  }

  public static MmuCall forModexpFullResultCopy(
      final Hub hub, final ModexpSubsection subsection, final ModexpMetadata modExpMetadata) {
    return new MmuCall(hub, MMU_INST_EXO_TO_RAM_TRANSPLANTS)
        .sourceId(subsection.exoModuleOperationId())
        .exoBytes(
            Optional.of(
                leftPadTo(subsection.returnDataRange.extract(), modExpMetadata.getMaxInputSize())))
        .targetId(subsection.returnDataContextNumber())
        .targetRamBytes(Optional.of(Bytes.EMPTY))
        .size(modExpMetadata.getMaxInputSize())
        .phase(PHASE_MODEXP_RESULT)
        .setBlakeModexp();
  }

  public static MmuCall forModexpPartialResultCopy(
      final Hub hub, final ModexpSubsection subsection, final ModexpMetadata modExpMetadata) {
    return new MmuCall(hub, MMU_INST_RAM_TO_RAM_SANS_PADDING)
        .sourceId(subsection.exoModuleOperationId())
        .sourceRamBytes(
            Optional.of(
                leftPadTo(subsection.returnDataRange.extract(), modExpMetadata.getMaxInputSize())))
        .targetId(hub.currentFrame().contextNumber())
        .targetRamBytes(Optional.of(subsection.rawCallerMemory()))
        .sourceOffset(EWord.of(modExpMetadata.getMaxInputSize() - modExpMetadata.mbs().toInt()))
        .size(modExpMetadata.mbs().toInt())
        .referenceOffset(subsection.returnAtOffset())
        .referenceSize(subsection.returnAtCapacity());
  }

  public static MmuCall mcopyCopy(Hub hub, CallFrame callFrame) {
    final EWord sourceOffset = EWord.of(callFrame.frame().getStackItem(1));
    final long size = clampedToLong(callFrame.frame().getStackItem(2));
    return new MmuCall(hub, MMU_INST_RAM_TO_RAM_SANS_PADDING)
        .sourceId(callFrame.contextNumber())
        .sourceRamBytes(
            Optional.of(
                extractContiguousLimbsFromMemory(
                    callFrame.frame(), Range.fromOffsetAndSize(sourceOffset.toLong(), size))))
        .targetId(newIdentifierFromStamp(hub.stamp()))
        .sourceOffset(sourceOffset)
        .size(size)
        .referenceSize(size);
  }

  public static MmuCall mcopyPaste(Hub hub, CallFrame callFrame) {
    final long size = clampedToLong(callFrame.frame().getStackItem(2));
    final long sourceOffset = clampedToLong(callFrame.frame().getStackItem(1));
    final Bytes currentMemory =
        hub.currentFrame().frame().shadowReadMemory(0, hub.currentFrame().frame().memoryByteSize());
    final Bytes slicedCurrentMemory =
        rightPaddedSlice(currentMemory, (int) sourceOffset, (int) size);
    return new MmuCall(hub, MMU_INST_RAM_TO_RAM_SANS_PADDING)
        .sourceId(newIdentifierFromStamp(hub.stamp()))
        .sourceRamBytes(Optional.of(slicedCurrentMemory))
        .targetId(callFrame.contextNumber())
        .targetRamBytes(Optional.of(currentMemory))
        .size(size)
        .referenceOffset(clampedToLong(callFrame.frame().getStackItem(0)))
        .referenceSize(size);
  }

  @Override
  public Trace.Hub traceHub(Trace.Hub trace, State hubState) {
    hubState.incrementMmuStamp();
    return trace
        .pMiscMmuFlag(true)
        .pMiscMmuInst(instruction)
        .pMiscMmuTgtId(targetId())
        .pMiscMmuSrcId(sourceId())
        .pMiscMmuAuxId(auxId())
        .pMiscMmuSrcOffsetHi(sourceOffset.hi())
        .pMiscMmuSrcOffsetLo(sourceOffset.lo())
        .pMiscMmuTgtOffsetLo(targetOffset.lo())
        .pMiscMmuSize(size)
        .pMiscMmuRefOffset(referenceOffset)
        .pMiscMmuRefSize(referenceSize())
        .pMiscMmuSuccessBit(successBit)
        .pMiscMmuLimb1(limb1)
        .pMiscMmuLimb2(limb2)
        .pMiscMmuExoSum(exoSum)
        .pMiscMmuPhase(phase);
  }

  @Override
  public void resolveAtEndTransaction(
      Hub hub, WorldView state, Transaction tx, boolean isSuccessful) {
    if (traceMe) {
      hub.mmu().call(this);
    }
  }
}
