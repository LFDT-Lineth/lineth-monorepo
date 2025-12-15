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

package net.consensys.linea.zktracer.module.mmu.instructions;

import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_LIMB_TRANSPLANT;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_LIMB_TWO_SOURCE;
import static net.consensys.linea.zktracer.Trace.Mmu.NB_MICRO_ROWS_TOT_MLOAD;
import static net.consensys.linea.zktracer.Trace.Mmu.NB_PP_ROWS_MLOAD;

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

public class MLoad implements MmuInstruction {
  private final Euc euc;
  private final Wcp wcp;
  private List<MmuEucCallRecord> eucCallRecords;
  private List<MmuWcpCallRecord> wcpCallRecords;

  private boolean aligned;
  private long initialSourceLimbOffset;
  private short initialSourceByteOffset;

  public MLoad(Euc euc, Wcp wcp) {
    this.euc = euc;
    this.wcp = wcp;
    this.eucCallRecords = new ArrayList<>(NB_PP_ROWS_MLOAD);
    this.wcpCallRecords = new ArrayList<>(NB_PP_ROWS_MLOAD);
  }

  public MmuData preProcess(MmuData mmuData) {
    final long dividend1 = mmuData.hubToMmuValues().sourceOffsetLo().longValueExact();
    final EucOperation eucOp = euc.callEUC(Bytes.ofUnsignedLong(dividend1), Bytes.of(LLARGE));
    final short rem = (short) eucOp.remainder().toInt();
    final long quot = eucOp.quotient().toLong();
    initialSourceLimbOffset = quot;
    initialSourceByteOffset = rem;

    eucCallRecords.add(
        MmuEucCallRecord.builder()
            .dividend(dividend1)
            .divisor((short) LLARGE)
            .quotient(quot)
            .remainder(rem)
            .build());

    final Bytes isZeroArg = Bytes.ofUnsignedInt(initialSourceByteOffset);
    final boolean result = wcp.callISZERO(isZeroArg);
    aligned = result;

    wcpCallRecords.add(
        MmuWcpCallRecord.instIsZeroBuilder().arg1Lo(isZeroArg).result(result).build());

    mmuData.eucCallRecords(eucCallRecords);
    mmuData.wcpCallRecords(wcpCallRecords);
    mmuData.outAndBinValues(MmuOutAndBinValues.DEFAULT);

    mmuData.totalLeftZeroesInitials(0);
    mmuData.totalNonTrivialInitials(NB_MICRO_ROWS_TOT_MLOAD);
    mmuData.totalRightZeroesInitials(0);

    return mmuData;
  }

  public MmuData setMicroInstructions(MmuData mmuData) {
    HubToMmuValues hubToMmuValues = mmuData.hubToMmuValues();

    // Setting MMIO constant values
    mmuData.mmuToMmioConstantValues(
        MmuToMmioConstantValues.builder().sourceContextNumber(hubToMmuValues.sourceId()).build());

    // First micro-instruction.
    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(
                aligned ? MMIO_INST_RAM_TO_LIMB_TRANSPLANT : MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
            .size((short) LLARGE)
            .sourceLimbOffset(initialSourceLimbOffset)
            .sourceByteOffset(initialSourceByteOffset)
            .limb(hubToMmuValues.limb1())
            .build());

    // Second micro-instruction.
    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(
                aligned ? MMIO_INST_RAM_TO_LIMB_TRANSPLANT : MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
            .size((short) LLARGE)
            .sourceLimbOffset(initialSourceLimbOffset + 1)
            .sourceByteOffset(initialSourceByteOffset)
            .limb(hubToMmuValues.limb2())
            .build());

    return mmuData;
  }
}
