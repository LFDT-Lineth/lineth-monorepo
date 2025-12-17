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
import static net.consensys.linea.zktracer.Trace.MMU_INST_INVALID_CODE_PREFIX;
import static net.consensys.linea.zktracer.Trace.MMU_INST_MLOAD;
import static net.consensys.linea.zktracer.Trace.MMU_INST_MODEXP_DATA;
import static net.consensys.linea.zktracer.Trace.MMU_INST_MODEXP_ZERO;
import static net.consensys.linea.zktracer.Trace.MMU_INST_MSTORE;
import static net.consensys.linea.zktracer.Trace.MMU_INST_MSTORE8;
import static net.consensys.linea.zktracer.Trace.MMU_INST_RAM_TO_EXO_WITH_PADDING;
import static net.consensys.linea.zktracer.Trace.MMU_INST_RAM_TO_RAM_SANS_PADDING;
import static net.consensys.linea.zktracer.Trace.MMU_INST_RIGHT_PADDED_WORD_EXTRACTION;

import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.module.euc.Euc;
import net.consensys.linea.zktracer.module.mmu.instructions.AnyToRamWithPadding;
import net.consensys.linea.zktracer.module.mmu.instructions.Blake;
import net.consensys.linea.zktracer.module.mmu.instructions.ExoToRamTransplants;
import net.consensys.linea.zktracer.module.mmu.instructions.InvalidCodePrefix;
import net.consensys.linea.zktracer.module.mmu.instructions.MLoad;
import net.consensys.linea.zktracer.module.mmu.instructions.MStore;
import net.consensys.linea.zktracer.module.mmu.instructions.MStore8;
import net.consensys.linea.zktracer.module.mmu.instructions.ModexpData;
import net.consensys.linea.zktracer.module.mmu.instructions.ModexpZero;
import net.consensys.linea.zktracer.module.mmu.instructions.RamToExoWithPadding;
import net.consensys.linea.zktracer.module.mmu.instructions.RamToRamSansPadding;
import net.consensys.linea.zktracer.module.mmu.instructions.RightPaddedWordExtraction;
import net.consensys.linea.zktracer.module.wcp.Wcp;

@Accessors(fluent = true)
class MmuInstructions {
  @Getter private final MLoad mLoadPreComputation;
  @Getter private final MStore mStorePreComputation;
  private final MStore8 mStore8PreComputation;
  private final InvalidCodePrefix invalidCodePrefix;
  private final RightPaddedWordExtraction rightPaddedWordExtraction;
  private final RamToExoWithPadding ramToExoWithPadding;
  private final ExoToRamTransplants exoToRamTransplants;
  private final RamToRamSansPadding ramToRamSansPadding;
  private final AnyToRamWithPadding anyToRamWithPadding;

  private final ModexpZero modexpZero;
  private final ModexpData modexpData;
  private final Blake blake;

  MmuInstructions(Euc euc, Wcp wcp) {
    this.mLoadPreComputation = new MLoad(euc, wcp);
    this.mStorePreComputation = new MStore(euc, wcp);
    this.mStore8PreComputation = new MStore8(euc);
    this.invalidCodePrefix = new InvalidCodePrefix(euc, wcp);
    this.rightPaddedWordExtraction = new RightPaddedWordExtraction(euc, wcp);
    this.ramToExoWithPadding = new RamToExoWithPadding(euc, wcp);
    this.exoToRamTransplants = new ExoToRamTransplants(euc);
    this.ramToRamSansPadding = new RamToRamSansPadding(euc, wcp);
    this.anyToRamWithPadding = new AnyToRamWithPadding(euc, wcp);
    this.modexpZero = new ModexpZero();
    this.modexpData = new ModexpData(euc, wcp);
    this.blake = new Blake(euc, wcp);
  }

  public MmuData compute(MmuData mmuData) {
    int mmuInstruction = mmuData.hubToMmuValues().mmuInstruction();

    return switch (mmuInstruction) {
      case MMU_INST_MLOAD -> {
        mmuData = mLoadPreComputation.preProcess(mmuData);
        yield mLoadPreComputation.setMicroInstructions(mmuData);
      }
      case MMU_INST_MSTORE -> {
        mmuData = mStorePreComputation.preProcess(mmuData);
        yield mStorePreComputation.setMicroInstructions(mmuData);
      }
      case MMU_INST_MSTORE8 -> {
        mmuData = mStore8PreComputation.preProcess(mmuData);
        yield mStore8PreComputation.setMicroInstructions(mmuData);
      }
      case MMU_INST_INVALID_CODE_PREFIX -> {
        mmuData = invalidCodePrefix.preProcess(mmuData);
        yield invalidCodePrefix.setMicroInstructions(mmuData);
      }
      case MMU_INST_RIGHT_PADDED_WORD_EXTRACTION -> {
        mmuData = rightPaddedWordExtraction.preProcess(mmuData);
        yield rightPaddedWordExtraction.setMicroInstructions(mmuData);
      }
      case MMU_INST_RAM_TO_EXO_WITH_PADDING -> {
        mmuData = ramToExoWithPadding.preProcess(mmuData);
        yield ramToExoWithPadding.setMicroInstructions(mmuData);
      }
      case MMU_INST_EXO_TO_RAM_TRANSPLANTS -> {
        mmuData = exoToRamTransplants.preProcess(mmuData);
        yield exoToRamTransplants.setMicroInstructions(mmuData);
      }
      case MMU_INST_RAM_TO_RAM_SANS_PADDING -> {
        mmuData = ramToRamSansPadding.preProcess(mmuData);
        yield ramToRamSansPadding.setMicroInstructions(mmuData);
      }
      case MMU_INST_ANY_TO_RAM_WITH_PADDING -> {
        mmuData = anyToRamWithPadding.preProcess(mmuData);
        yield anyToRamWithPadding.setMicroInstructions(mmuData);
      }
      case MMU_INST_MODEXP_ZERO -> {
        mmuData = modexpZero.preProcess(mmuData);
        yield modexpZero.setMicroInstructions(mmuData);
      }
      case MMU_INST_MODEXP_DATA -> {
        mmuData = modexpData.preProcess(mmuData);
        yield modexpData.setMicroInstructions(mmuData);
      }
      case MMU_INST_BLAKE -> {
        mmuData = blake.preProcess(mmuData);
        yield blake.setMicroInstructions(mmuData);
      }
      default ->
          throw new IllegalArgumentException(
              "Unexpected MMU instruction: %d".formatted(mmuInstruction));
    };
  }
}
