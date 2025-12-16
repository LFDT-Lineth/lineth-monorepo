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
import static net.consensys.linea.zktracer.module.hub.Hub.newIdentifierFromStamp;
import static net.consensys.linea.zktracer.runtime.callstack.CallFrame.extractContiguousLimbsFromMemory;
import static org.hyperledger.besu.evm.internal.Words.clampedToLong;

import java.util.Optional;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall;
import net.consensys.linea.zktracer.module.romlex.ContractMetadata;
import net.consensys.linea.zktracer.module.romlex.RomLexDefer;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import net.consensys.linea.zktracer.types.EWord;
import net.consensys.linea.zktracer.types.Range;
import org.apache.tuweni.bytes.Bytes;

/**
 * A specialization of {@link MmuCall} that addresses the fact that the MMU requires access to the
 * sorted Code Fragment Index of the created contract, which is only available post-conflation.
 */
public class Create2 extends MmuCall implements RomLexDefer {
  private final Hub hub;
  private ContractMetadata contract;

  public Create2(final Hub hub, final Bytes create2initCode, final boolean failedCreate) {
    super(hub, MMU_INST_RAM_TO_EXO_WITH_PADDING);
    this.hub = hub;
    this.hub.romLex().createDefers().register(this);

    final CallFrame currentFrame = hub.currentFrame();
    final Bytes sourceOffset = currentFrame.frame().getStackItem(1);
    final Bytes size = currentFrame.frame().getStackItem(2);

    this.sourceId(currentFrame.contextNumber())
        .sourceRamBytes(
            Optional.of(
                extractContiguousLimbsFromMemory(
                    currentFrame.frame(), Range.fromOffsetAndSize(sourceOffset, size))))
        .auxId(newIdentifierFromStamp(hub.stamp()))
        .exoBytes(Optional.of(create2initCode))
        .sourceOffset(EWord.of(sourceOffset))
        .size(clampedToLong(size))
        .referenceSize(clampedToLong(size))
        .setKec();

    if (!failedCreate) {
      this.setRom();
    }
  }

  @Override
  public int targetId() {
    return exoIsRom ? hub.romLex().getCodeFragmentIndexByMetadata(contract) : 0;
  }

  @Override
  public void updateContractMetadata(ContractMetadata metadata) {
    contract = metadata;
  }
}
