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

package net.consensys.linea.zktracer.module.mmio;

import static java.util.Map.entry;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_LIMB_TO_RAM_ONE_TARGET;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_LIMB_TO_RAM_TRANSPLANT;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_LIMB_TO_RAM_TWO_TARGET;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_LIMB_VANISHES;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_EXCISION;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_LIMB_ONE_SOURCE;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_LIMB_TRANSPLANT;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_LIMB_TWO_SOURCE;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_RAM_PARTIAL;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_RAM_TRANSPLANT;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_RAM_TWO_SOURCE;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_RAM_TWO_TARGET;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_VANISHES;

import java.util.Map;

import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.module.mmio.instructions.LimbToRamOneTarget;
import net.consensys.linea.zktracer.module.mmio.instructions.LimbToRamTransplant;
import net.consensys.linea.zktracer.module.mmio.instructions.LimbToRamTwoTarget;
import net.consensys.linea.zktracer.module.mmio.instructions.LimbVanishes;
import net.consensys.linea.zktracer.module.mmio.instructions.MmioInstruction;
import net.consensys.linea.zktracer.module.mmio.instructions.RamExcision;
import net.consensys.linea.zktracer.module.mmio.instructions.RamToLimbOneSource;
import net.consensys.linea.zktracer.module.mmio.instructions.RamToLimbTransplant;
import net.consensys.linea.zktracer.module.mmio.instructions.RamToLimbTwoSource;
import net.consensys.linea.zktracer.module.mmio.instructions.RamToRamPartial;
import net.consensys.linea.zktracer.module.mmio.instructions.RamToRamTransplant;
import net.consensys.linea.zktracer.module.mmio.instructions.RamToRamTwoSource;
import net.consensys.linea.zktracer.module.mmio.instructions.RamToRamTwoTarget;
import net.consensys.linea.zktracer.module.mmio.instructions.RamVanishes;
import net.consensys.linea.zktracer.module.mmu.MmuData;

@Accessors(fluent = true)
public class MmioInstructions {
  private final Map<Integer, MmioInstruction> mmioInstructionMap;

  public MmioInstructions(final MmuData mmuData, final int mmioInstructionNumber) {
    mmioInstructionMap =
        Map.ofEntries(
            entry(MMIO_INST_LIMB_VANISHES, new LimbVanishes(mmuData, mmioInstructionNumber)),
            entry(
                MMIO_INST_LIMB_TO_RAM_TRANSPLANT,
                new LimbToRamTransplant(mmuData, mmioInstructionNumber)),
            entry(
                MMIO_INST_LIMB_TO_RAM_ONE_TARGET,
                new LimbToRamOneTarget(mmuData, mmioInstructionNumber)),
            entry(
                MMIO_INST_LIMB_TO_RAM_TWO_TARGET,
                new LimbToRamTwoTarget(mmuData, mmioInstructionNumber)),
            entry(
                MMIO_INST_RAM_TO_LIMB_TRANSPLANT,
                new RamToLimbTransplant(mmuData, mmioInstructionNumber)),
            entry(
                MMIO_INST_RAM_TO_LIMB_ONE_SOURCE,
                new RamToLimbOneSource(mmuData, mmioInstructionNumber)),
            entry(
                MMIO_INST_RAM_TO_LIMB_TWO_SOURCE,
                new RamToLimbTwoSource(mmuData, mmioInstructionNumber)),
            entry(
                MMIO_INST_RAM_TO_RAM_TRANSPLANT,
                new RamToRamTransplant(mmuData, mmioInstructionNumber)),
            entry(
                MMIO_INST_RAM_TO_RAM_PARTIAL, new RamToRamPartial(mmuData, mmioInstructionNumber)),
            entry(
                MMIO_INST_RAM_TO_RAM_TWO_TARGET,
                new RamToRamTwoTarget(mmuData, mmioInstructionNumber)),
            entry(MMIO_INST_RAM_EXCISION, new RamExcision(mmuData, mmioInstructionNumber)),
            entry(
                MMIO_INST_RAM_TO_RAM_TWO_SOURCE,
                new RamToRamTwoSource(mmuData, mmioInstructionNumber)),
            entry(MMIO_INST_RAM_VANISHES, new RamVanishes(mmuData, mmioInstructionNumber)));
  }

  public MmioData compute(final int mmioInstruction) {
    return mmioInstructionMap.get(mmioInstruction).execute();
  }
}
