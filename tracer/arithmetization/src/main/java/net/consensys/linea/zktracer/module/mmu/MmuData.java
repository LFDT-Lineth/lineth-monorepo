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

package net.consensys.linea.zktracer.module.mmu;

import static net.consensys.linea.zktracer.Trace.MMU_INST_ANY_TO_RAM_WITH_PADDING;
import static net.consensys.linea.zktracer.Trace.MMU_INST_BLAKE;
import static net.consensys.linea.zktracer.Trace.MMU_INST_EXO_TO_RAM_TRANSPLANTS;
import static net.consensys.linea.zktracer.Trace.MMU_INST_MODEXP_DATA;
import static net.consensys.linea.zktracer.Trace.MMU_INST_MODEXP_ZERO;
import static net.consensys.linea.zktracer.Trace.MMU_INST_RAM_TO_EXO_WITH_PADDING;

import java.util.ArrayList;
import java.util.List;

import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.Setter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall;
import net.consensys.linea.zktracer.module.mmu.values.HubToMmuValues;
import net.consensys.linea.zktracer.module.mmu.values.MmuEucCallRecord;
import net.consensys.linea.zktracer.module.mmu.values.MmuOutAndBinValues;
import net.consensys.linea.zktracer.module.mmu.values.MmuToMmioConstantValues;
import net.consensys.linea.zktracer.module.mmu.values.MmuToMmioInstruction;
import net.consensys.linea.zktracer.module.mmu.values.MmuWcpCallRecord;
import org.apache.tuweni.bytes.Bytes;

@AllArgsConstructor
@Getter
@Setter
@Accessors(fluent = true)
public class MmuData {
  private final MmuCall mmuCall;
  private int totalLeftZeroesInitials;
  private int totalRightZeroesInitials;
  private int totalNonTrivialInitials;
  private List<MmuEucCallRecord> eucCallRecords;
  private List<MmuWcpCallRecord> wcpCallRecords;
  private MmuOutAndBinValues outAndBinValues;
  private HubToMmuValues hubToMmuValues;
  private MmuToMmioConstantValues mmuToMmioConstantValues;
  private List<MmuToMmioInstruction> mmuToMmioInstructions;
  private boolean mmuInstAnyToRamWithPaddingIsPurePadding;
  private Bytes exoBytes;
  private Bytes sourceRamBytes;
  private Bytes targetRamBytes;
  private final boolean exoLimbIsSource;
  private final boolean exoLimbIsTarget;
  private static final List<Integer> MMU_INST_EXO_IS_SOURCE =
      List.of(MMU_INST_ANY_TO_RAM_WITH_PADDING, MMU_INST_EXO_TO_RAM_TRANSPLANTS);
  private static final List<Integer> MMU_INST_EXO_IS_TARGET =
      List.of(
          MMU_INST_BLAKE,
          MMU_INST_MODEXP_DATA,
          MMU_INST_MODEXP_ZERO,
          MMU_INST_RAM_TO_EXO_WITH_PADDING);

  public MmuData(final MmuCall mmuCall) {
    this(
        mmuCall,
        0,
        0,
        0,
        new ArrayList<>(),
        new ArrayList<>(),
        MmuOutAndBinValues.DEFAULT,
        null,
        MmuToMmioConstantValues.builder().build(),
        new ArrayList<>(),
        false,
        Bytes.EMPTY,
        Bytes.EMPTY,
        Bytes.EMPTY,
        MMU_INST_EXO_IS_SOURCE.contains(mmuCall.instruction()),
        MMU_INST_EXO_IS_TARGET.contains(mmuCall.instruction()));

    this.setSourceRamBytes();
    this.setTargetRamBytes();
    this.setExoBytes();
  }

  public int numberMmioInstructions() {
    return totalLeftZeroesInitials + totalNonTrivialInitials + totalRightZeroesInitials;
  }

  public int numberMmuPreprocessingRows() {
    return wcpCallRecords().size();
  }

  public void mmuToMmioInstruction(final MmuToMmioInstruction mmuToMmioInstruction) {
    mmuToMmioInstructions.add(mmuToMmioInstruction);
  }

  public void setSourceRamBytes() {
    if (mmuCall.sourceRamBytes().isPresent()) {
      sourceRamBytes(mmuCall.sourceRamBytes().get());
    }
  }

  public void setTargetRamBytes() {
    if (mmuCall.targetRamBytes().isPresent()) {
      targetRamBytes(mmuCall.targetRamBytes().get());
    }
  }

  public void setExoBytes() {
    if (mmuCall.exoBytes().isPresent()) {
      exoBytes(mmuCall.exoBytes().get());
    }
  }
}
