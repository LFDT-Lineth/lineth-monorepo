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

package net.consensys.linea.zktracer.module.mmu.instructions;

import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_LIMB_ONE_SOURCE;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_LIMB_TWO_SOURCE;
import static net.consensys.linea.zktracer.Trace.Mmu.NB_MICRO_ROWS_TOT_BLAKE;
import static net.consensys.linea.zktracer.Trace.Mmu.NB_PP_ROWS_BLAKE;
import static net.consensys.linea.zktracer.types.Conversions.longToBytes;

import java.util.ArrayList;
import java.util.List;

import net.consensys.linea.zktracer.module.euc.Euc;
import net.consensys.linea.zktracer.module.euc.EucOperation;
import net.consensys.linea.zktracer.module.mmu.MmuData;
import net.consensys.linea.zktracer.module.mmu.values.HubToMmuValues;
import net.consensys.linea.zktracer.module.mmu.values.MmuEucCallRecord;
import net.consensys.linea.zktracer.module.mmu.values.MmuOutAndBinValues;
import net.consensys.linea.zktracer.module.mmu.values.MmuToMmioConstantValues;
import net.consensys.linea.zktracer.module.mmu.values.MmuToMmioInstruction;
import net.consensys.linea.zktracer.module.mmu.values.MmuWcpCallRecord;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import org.apache.tuweni.bytes.Bytes;

public class Blake implements MmuInstruction {
  private final Euc euc;
  private final Wcp wcp;
  private final List<MmuEucCallRecord> eucCallRecords;
  private final List<MmuWcpCallRecord> wcpCallRecords;
  private long sourceLimbOffsetR;
  private short sourceByteOffsetR;
  private long sourceLimbOffsetF;
  private short sourceByteOffsetF;
  private boolean blakeRSingleSource;

  public Blake(Euc euc, Wcp wcp) {
    this.euc = euc;
    this.wcp = wcp;
    this.eucCallRecords = new ArrayList<>(NB_PP_ROWS_BLAKE);
    this.wcpCallRecords = new ArrayList<>(NB_PP_ROWS_BLAKE);
  }

  @Override
  public MmuData preProcess(MmuData mmuData) {
    final HubToMmuValues hubToMmuValues = mmuData.hubToMmuValues();

    // Preprocessing row n°1
    final long dividendRow1 = hubToMmuValues.sourceOffsetLo().longValueExact();
    final EucOperation eucOpRow1 = euc.callEUC(longToBytes(dividendRow1), Bytes.of(LLARGE));
    sourceLimbOffsetR = eucOpRow1.quotient().toLong();
    sourceByteOffsetR = (short) eucOpRow1.remainder().toInt();
    eucCallRecords.add(
        MmuEucCallRecord.builder()
            .dividend(dividendRow1)
            .divisor((short) LLARGE)
            .quotient(eucOpRow1.quotient().toLong())
            .remainder((short) eucOpRow1.remainder().toInt())
            .build());

    final Bytes wcpArg1 = longToBytes(sourceByteOffsetR + 3);
    final Bytes wcpArg2 = Bytes.of(LLARGE);
    blakeRSingleSource = wcp.callLT(wcpArg1, wcpArg2);
    wcpCallRecords.add(
        MmuWcpCallRecord.instLtBuilder()
            .arg1Lo(wcpArg1)
            .arg2Lo(wcpArg2)
            .result(blakeRSingleSource)
            .build());

    // Preprocessing row n°2
    final long dividendRow2 = hubToMmuValues.sourceOffsetLo().longValueExact() + 213 - 1;
    final EucOperation eucOpRow2 = euc.callEUC(longToBytes(dividendRow2), Bytes.of(LLARGE));
    sourceLimbOffsetF = eucOpRow2.quotient().toLong();
    sourceByteOffsetF = (short) eucOpRow2.remainder().toInt();
    eucCallRecords.add(
        MmuEucCallRecord.builder()
            .dividend(dividendRow2)
            .divisor((short) LLARGE)
            .quotient(eucOpRow2.quotient().toLong())
            .remainder((short) eucOpRow2.remainder().toInt())
            .build());

    wcpCallRecords.add(MmuWcpCallRecord.EMPTY_CALL); // no second call to wcp

    mmuData.eucCallRecords(eucCallRecords);
    mmuData.wcpCallRecords(wcpCallRecords);

    // setting Out and Bin values
    mmuData.outAndBinValues(MmuOutAndBinValues.DEFAULT); // all value to 0

    // setting the number of micro instruction rows
    mmuData.totalLeftZeroesInitials(0);
    mmuData.totalNonTrivialInitials(NB_MICRO_ROWS_TOT_BLAKE);
    mmuData.totalRightZeroesInitials(0);

    return mmuData;
  }

  @Override
  public MmuData setMicroInstructions(MmuData mmuData) {
    HubToMmuValues hubToMmuValues = mmuData.hubToMmuValues();

    // Common values
    final boolean successBit = hubToMmuValues.successBit();
    mmuData.mmuToMmioConstantValues(
        MmuToMmioConstantValues.builder()
            .sourceContextNumber(hubToMmuValues.sourceId())
            .successBit(successBit)
            .exoSum(successBit ? hubToMmuValues.exoSum() : 0)
            .phase(successBit ? hubToMmuValues.phase() : 0)
            .exoId(successBit ? (int) hubToMmuValues.targetId() : 0)
            .build());

    // First micro instruction
    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(
                blakeRSingleSource
                    ? MMIO_INST_RAM_TO_LIMB_ONE_SOURCE
                    : MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
            .size((short) 4)
            .sourceLimbOffset(sourceLimbOffsetR)
            .sourceByteOffset(sourceByteOffsetR)
            .targetByteOffset((short) (LLARGE - 4))
            .targetLimbOffset(0)
            .limb(hubToMmuValues.limb1())
            .build());

    // Second micro instruction
    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(MMIO_INST_RAM_TO_LIMB_ONE_SOURCE)
            .size((short) 1)
            .sourceLimbOffset(sourceLimbOffsetF)
            .sourceByteOffset(sourceByteOffsetF)
            .targetByteOffset((short) (LLARGE - 1))
            .targetLimbOffset(1)
            .limb(hubToMmuValues.limb2())
            .build());

    return mmuData;
  }
}
