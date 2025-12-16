/*
 * Copyright ConsenSys AG.
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

package net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.opcode;

import static net.consensys.linea.zktracer.Trace.MMU_INST_RAM_TO_EXO_WITH_PADDING;

import java.util.Optional;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall;
import net.consensys.linea.zktracer.module.romlex.ContractMetadata;
import net.consensys.linea.zktracer.module.romlex.RomLexDefer;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.internal.Words;

/**
 * A specialization of {@link MmuCall} that addresses the fact that the MMU requires access to the
 * sorted Code Fragment Index of the created contract, which is only available post-conflation.
 */
public class Create extends MmuCall implements RomLexDefer {
  private final Hub hub;
  private ContractMetadata contract;

  public Create(final Hub hub) {
    super(hub, MMU_INST_RAM_TO_EXO_WITH_PADDING);
    this.hub = hub;
    this.hub.romLex().createDefers().register(this);

    this.sourceId(hub.currentFrame().contextNumber())
        .sourceRamBytes(
            Optional.of(
                hub.currentFrame()
                    .frame()
                    .shadowReadMemory(0, hub.currentFrame().frame().memoryByteSize())))
        .sourceOffset(EWord.of(hub.messageFrame().getStackItem(1)))
        .size(Words.clampedToLong(hub.messageFrame().getStackItem(2)))
        .referenceSize(Words.clampedToLong(hub.messageFrame().getStackItem(2)))
        .setRom();
  }

  @Override
  public int targetId() {
    return hub.romLex().getCodeFragmentIndexByMetadata(contract);
  }

  @Override
  public Optional<Bytes> exoBytes() {
    return Optional.of(hub.romLex().getCodeByMetadata(contract));
  }

  @Override
  public void updateContractMetadata(ContractMetadata metadata) {
    contract = metadata;
  }
}
