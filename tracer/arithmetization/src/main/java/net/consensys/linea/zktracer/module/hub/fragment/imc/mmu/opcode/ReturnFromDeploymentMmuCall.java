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

import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall;
import net.consensys.linea.zktracer.module.romlex.ContractMetadata;
import net.consensys.linea.zktracer.module.shakiradata.ShakiraDataOperation;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import net.consensys.linea.zktracer.types.EWord;
import net.consensys.linea.zktracer.types.Range;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;

/**
 * A specialization of {@link MmuCall} that addresses the fact that the MMU requires access to the
 * sorted Code Fragment Index of the deployed bytecode, which is only available post-conflation.
 */
@Accessors(fluent = true)
public class ReturnFromDeploymentMmuCall extends MmuCall {
  private final Hub hub;
  private ContractMetadata contract;
  @Getter private final Bytes32 hashResult;

  public ReturnFromDeploymentMmuCall(final Hub hub) {
    super(hub, MMU_INST_RAM_TO_EXO_WITH_PADDING);

    this.hub = hub;

    final CallFrame currentFrame = hub.currentFrame();
    final Address contractAddress = currentFrame.frame().getContractAddress();
    final int depNumber = hub.deploymentNumberOf(contractAddress);
    contract = ContractMetadata.make(contractAddress, depNumber, false);

    final ShakiraDataOperation shakiraDataOperation =
        new ShakiraDataOperation(hub.stamp(), hub.romLex().byteCode());
    hub.shakiraData().call(shakiraDataOperation);

    hashResult = shakiraDataOperation.result();

    final Bytes sourceOffset = currentFrame.frame().getStackItem(0);
    final Bytes size = currentFrame.frame().getStackItem(1);

    this.sourceId(currentFrame.contextNumber())
        .sourceRamBytes(
            Optional.of(
                extractContiguousLimbsFromMemory(
                    currentFrame.frame(), Range.fromOffsetAndSize(sourceOffset, size))))
        .auxId(newIdentifierFromStamp(hub.stamp()))
        .sourceOffset(EWord.of(sourceOffset))
        .size(clampedToLong(size))
        .referenceSize(clampedToLong(size))
        .setKec()
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
}
