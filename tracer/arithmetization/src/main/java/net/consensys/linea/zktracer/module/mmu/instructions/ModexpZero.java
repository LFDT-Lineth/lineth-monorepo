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

import static net.consensys.linea.zktracer.Trace.MMIO_INST_LIMB_VANISHES;
import static net.consensys.linea.zktracer.Trace.Mmu.NB_MICRO_ROWS_TOT_MODEXP_ZERO;
import static net.consensys.linea.zktracer.Trace.Mmu.NB_PP_ROWS_MODEXP_ZERO;

import java.util.ArrayList;
import java.util.List;
import net.consensys.linea.zktracer.module.mmu.MmuData;
import net.consensys.linea.zktracer.module.mmu.values.HubToMmuValues;
import net.consensys.linea.zktracer.module.mmu.values.MmuEucCallRecord;
import net.consensys.linea.zktracer.module.mmu.values.MmuOutAndBinValues;
import net.consensys.linea.zktracer.module.mmu.values.MmuToMmioConstantValues;
import net.consensys.linea.zktracer.module.mmu.values.MmuToMmioInstruction;
import net.consensys.linea.zktracer.module.mmu.values.MmuWcpCallRecord;

public class ModexpZero implements MmuInstruction {
  private final List<MmuEucCallRecord> eucCallRecords;
  private final List<MmuWcpCallRecord> wcpCallRecords;

  public ModexpZero() {
    this.eucCallRecords = new ArrayList<>(NB_PP_ROWS_MODEXP_ZERO);
    this.wcpCallRecords = new ArrayList<>(NB_PP_ROWS_MODEXP_ZERO);
  }

  @Override
  public MmuData preProcess(MmuData mmuData) {

    // no call to wcp nor euc. So much fun.
    eucCallRecords.add(MmuEucCallRecord.EMPTY_CALL);
    wcpCallRecords.add(MmuWcpCallRecord.EMPTY_CALL);
    mmuData.eucCallRecords(eucCallRecords);
    mmuData.wcpCallRecords(wcpCallRecords);

    // setting Out and Bin values
    mmuData.outAndBinValues(MmuOutAndBinValues.builder().build()); // all 0. Fun is at its peak.

    mmuData.totalLeftZeroesInitials(0);
    mmuData.totalNonTrivialInitials(NB_MICRO_ROWS_TOT_MODEXP_ZERO);
    mmuData.totalRightZeroesInitials(0);

    return mmuData;
  }

  @Override
  public MmuData setMicroInstructions(MmuData mmuData) {
    HubToMmuValues hubToMmuValues = mmuData.hubToMmuValues();

    // Setting MMIO constant values
    mmuData.mmuToMmioConstantValues(
        MmuToMmioConstantValues.builder()
            .exoSum(hubToMmuValues.exoSum())
            .phase(hubToMmuValues.phase())
            .exoId((int) hubToMmuValues.targetId())
            .build());

    for (int i = 0; i < NB_MICRO_ROWS_TOT_MODEXP_ZERO; i++) {
      vanishingMicroInstruction(mmuData, i);
    }

    return mmuData;
  }

  private void vanishingMicroInstruction(MmuData mmuData, final int i) {
    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(MMIO_INST_LIMB_VANISHES)
            .targetLimbOffset(i)
            .build());
  }
}
