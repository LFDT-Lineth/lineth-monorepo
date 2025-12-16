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
import static net.consensys.linea.zktracer.Trace.MMIO_INST_LIMB_TO_RAM_TRANSPLANT;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_LIMB_TO_RAM_TWO_TARGET;
import static net.consensys.linea.zktracer.Trace.Mmu.NB_MICRO_ROWS_TOT_MSTORE;
import static net.consensys.linea.zktracer.Trace.Mmu.NB_PP_ROWS_MSTORE;

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

public class MStore implements MmuInstruction {
  private final Euc euc;
  private final Wcp wcp;
  private List<MmuEucCallRecord> eucCallRecords;
  private List<MmuWcpCallRecord> wcpCallRecords;

  private boolean aligned;
  private long initialTargetLimbOffset;
  private short initialTargetByteOffset;

  public MStore(Euc euc, Wcp wcp) {
    this.euc = euc;
    this.wcp = wcp;
    this.eucCallRecords = new ArrayList<>(NB_PP_ROWS_MSTORE);
    this.wcpCallRecords = new ArrayList<>(NB_PP_ROWS_MSTORE);
  }

  @Override
  public MmuData preProcess(MmuData mmuData) {

    // row n°1
    final long dividend = mmuData.hubToMmuValues().targetOffset();
    final EucOperation eucOp = euc.callEUC(Bytes.ofUnsignedLong(dividend), Bytes.of(16));
    final short rem = (short) eucOp.remainder().toInt();
    final long quot = eucOp.quotient().toLong();
    initialTargetLimbOffset = quot;
    initialTargetByteOffset = rem;

    eucCallRecords.add(
        MmuEucCallRecord.builder()
            .dividend(dividend)
            .divisor((short) LLARGE)
            .quotient(quot)
            .remainder(rem)
            .build());

    Bytes isZeroArg = Bytes.ofUnsignedInt(initialTargetByteOffset);
    boolean result = wcp.callISZERO(isZeroArg);
    aligned = result;

    wcpCallRecords.add(
        MmuWcpCallRecord.instIsZeroBuilder()
            .arg1Hi(Bytes.EMPTY)
            .arg1Lo(isZeroArg)
            .arg2Lo(Bytes.EMPTY)
            .result(result)
            .build());

    mmuData.eucCallRecords(eucCallRecords);
    mmuData.wcpCallRecords(wcpCallRecords);

    // setting Out and Bin values
    mmuData.outAndBinValues(MmuOutAndBinValues.builder().build()); // all 0

    mmuData.totalLeftZeroesInitials(0);
    mmuData.totalNonTrivialInitials(NB_MICRO_ROWS_TOT_MSTORE);
    mmuData.totalRightZeroesInitials(0);

    return mmuData;
  }

  @Override
  public MmuData setMicroInstructions(MmuData mmuData) {
    HubToMmuValues hubToMmuValues = mmuData.hubToMmuValues();

    // Setting MMIO constant values
    mmuData.mmuToMmioConstantValues(
        MmuToMmioConstantValues.builder().targetContextNumber(hubToMmuValues.targetId()).build());

    // First micro-instruction.
    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(
                aligned ? MMIO_INST_LIMB_TO_RAM_TRANSPLANT : MMIO_INST_LIMB_TO_RAM_TWO_TARGET)
            .size((short) LLARGE)
            .targetLimbOffset(initialTargetLimbOffset)
            .targetByteOffset(initialTargetByteOffset)
            .limb(hubToMmuValues.limb1())
            .targetLimbIsTouchedTwice(!aligned)
            .build());

    // Second micro-instruction.
    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(
                aligned ? MMIO_INST_LIMB_TO_RAM_TRANSPLANT : MMIO_INST_LIMB_TO_RAM_TWO_TARGET)
            .size((short) LLARGE)
            .targetLimbOffset(initialTargetLimbOffset + 1)
            .targetByteOffset(initialTargetByteOffset)
            .limb(hubToMmuValues.limb2())
            .targetLimbIsTouchedTwice(!aligned)
            .build());

    return mmuData;
  }
}
