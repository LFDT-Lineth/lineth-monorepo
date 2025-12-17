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

import static net.consensys.linea.zktracer.Trace.EIP_3541_MARKER;
import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.Trace.LLARGEMO;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_LIMB_ONE_SOURCE;
import static net.consensys.linea.zktracer.Trace.Mmu.NB_MICRO_ROWS_TOT_INVALID_CODE_PREFIX;
import static net.consensys.linea.zktracer.Trace.Mmu.NB_PP_ROWS_INVALID_CODE_PREFIX;
import static net.consensys.linea.zktracer.types.Utils.leftPadToBytes16;

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

public class InvalidCodePrefix implements MmuInstruction {
  private final Euc euc;
  private final Wcp wcp;
  private final List<MmuEucCallRecord> eucCallRecords;
  private final List<MmuWcpCallRecord> wcpCallRecords;

  private long initialSourceLimbOffset;
  private short initialSourceByteOffset;
  private Bytes microLimb;

  public InvalidCodePrefix(Euc euc, Wcp wcp) {
    this.euc = euc;
    this.wcp = wcp;
    this.eucCallRecords = new ArrayList<>(NB_PP_ROWS_INVALID_CODE_PREFIX);
    this.wcpCallRecords = new ArrayList<>(NB_PP_ROWS_INVALID_CODE_PREFIX);
  }

  @Override
  public MmuData preProcess(MmuData mmuData) {
    mmuData.sourceRamBytes(mmuData.mmuCall().sourceRamBytes().get());

    // row n°1
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

    final long offset = mmuData.hubToMmuValues().sourceOffsetLo().longValueExact();
    final boolean offsetIsOutOfBonds = offset >= mmuData.sourceRamBytes().size();
    microLimb =
        offsetIsOutOfBonds ? Bytes.of(0) : Bytes.of(mmuData.sourceRamBytes().get((int) offset));
    final Bytes arg1 = microLimb;
    final Bytes arg2 = Bytes.of(EIP_3541_MARKER);
    final boolean result = wcp.callEQ(arg1, arg2);

    wcpCallRecords.add(
        MmuWcpCallRecord.instEqBuilder().arg1Lo(arg1).arg2Lo(arg2).result(result).build());

    mmuData.eucCallRecords(eucCallRecords);
    mmuData.wcpCallRecords(wcpCallRecords);

    // setting Out and Bin values
    mmuData.outAndBinValues(MmuOutAndBinValues.builder().build()); // all 0

    mmuData.totalLeftZeroesInitials(0);
    mmuData.totalNonTrivialInitials(NB_MICRO_ROWS_TOT_INVALID_CODE_PREFIX);
    mmuData.totalRightZeroesInitials(0);

    return mmuData;
  }

  @Override
  public MmuData setMicroInstructions(MmuData mmuData) {
    final HubToMmuValues hubToMmuValues = mmuData.hubToMmuValues();

    mmuData.mmuToMmioConstantValues(
        MmuToMmioConstantValues.builder()
            .sourceContextNumber(hubToMmuValues.sourceId())
            .successBit(hubToMmuValues.successBit())
            .build());

    // First and only micro-instruction.
    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(MMIO_INST_RAM_TO_LIMB_ONE_SOURCE)
            .size((short) 1)
            .sourceLimbOffset(initialSourceLimbOffset)
            .sourceByteOffset(initialSourceByteOffset)
            .targetByteOffset((short) LLARGEMO)
            .limb(leftPadToBytes16(microLimb))
            .build());

    return mmuData;
  }
}
