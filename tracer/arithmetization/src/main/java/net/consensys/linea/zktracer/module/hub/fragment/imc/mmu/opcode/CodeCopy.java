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

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.Trace.MMU_INST_ANY_TO_RAM_WITH_PADDING;
import static net.consensys.linea.zktracer.runtime.callstack.CallFrame.extractContiguousLimbsFromMemory;
import static org.hyperledger.besu.evm.internal.Words.clampedToLong;

import java.util.Optional;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall;
import net.consensys.linea.zktracer.module.romlex.ContractMetadata;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import net.consensys.linea.zktracer.types.EWord;
import net.consensys.linea.zktracer.types.Range;
import org.apache.tuweni.bytes.Bytes;

/**
 * A specialization of {@link MmuCall} that addresses the fact that the MMU requires access to the
 * sorted Code Fragment Index of the copied bytecode, which is only available post-conflation.
 */
public class CodeCopy extends MmuCall {
  private final Hub hub;
  private final ContractMetadata contract;

  public CodeCopy(final Hub hub) {
    super(hub, MMU_INST_ANY_TO_RAM_WITH_PADDING);
    this.hub = hub;
    this.contract = hub.currentFrame().metadata();
    final CallFrame currentFrame = hub.currentFrame();
    final Bytes targetOffset = currentFrame.frame().getStackItem(0);
    final Bytes sourceOffset = currentFrame.frame().getStackItem(1);
    final Bytes size = currentFrame.frame().getStackItem(2);

    // the MMU module only deals with nontrivial CODECOPY instructions
    checkArgument(!size.isZero(), "CODECOPY: size 0 operations should not trigger the MMU module");

    this.exoBytes(Optional.of(currentFrame.code().bytecode()))
        .targetId(currentFrame.contextNumber())
        .targetRamBytes(
            Optional.of(
                extractContiguousLimbsFromMemory(
                    currentFrame.frame(), Range.fromOffsetAndSize(targetOffset, size))))
        .sourceOffset(EWord.of(sourceOffset))
        .targetOffset(EWord.of(targetOffset))
        .size(clampedToLong(size))
        .referenceSize(currentFrame.code().getSize())
        .setRom();
  }

  @Override
  public int sourceId() {
    return hub.romLex().getCodeFragmentIndexByMetadata(contract);
  }
}
