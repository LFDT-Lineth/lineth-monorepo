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

package net.consensys.linea.zktracer.module.shakiradata;

import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.module.ModuleName.SHAKIRA_DATA;

import java.util.List;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.OperationListModule;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationStackedList;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.module.limits.Keccak;
import net.consensys.linea.zktracer.module.limits.precompiles.RipemdBlocks;
import net.consensys.linea.zktracer.module.limits.precompiles.Sha256Blocks;
import net.consensys.linea.zktracer.module.wcp.Wcp;

@RequiredArgsConstructor
@Accessors(fluent = true)
public class ShakiraData implements OperationListModule<ShakiraDataOperation> {
  @Getter
  private final ModuleOperationStackedList<ShakiraDataOperation> operations =
      new ModuleOperationStackedList<>();

  private final Wcp wcp;

  private final Sha256Blocks sha256Blocks;
  private final Keccak keccak;
  private final RipemdBlocks ripemdBlocks;

  private long previousID = 0;

  @Override
  public ModuleName moduleKey() {
    return SHAKIRA_DATA;
  }

  @Override
  public int lineCount() {
    return operations.lineCount();
  }

  @Override
  public int spillage(Trace trace) {
    return trace.shakiradata().spillage();
  }

  public void call(final ShakiraDataOperation operation) {
    operations.add(operation);

    wcp.callLT(previousID, operation.ID());
    previousID = operation.ID();
    wcp.callGT(operation.lastNBytes(), 0);
    wcp.callLEQ(operation.lastNBytes(), LLARGE);

    switch (operation.hashType()) {
      case SHA256 -> sha256Blocks.updateTally(operation.inputSize());
      case KECCAK -> keccak.updateTally(operation.inputSize());
      case RIPEMD -> ripemdBlocks.updateTally(operation.inputSize());
      default -> throw new IllegalArgumentException("Precompile type not supported by SHAKIRA");
    }
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.shakiradata().headers(this.lineCount());
  }

  @Override
  public void commit(Trace trace) {
    int stamp = 0;
    for (ShakiraDataOperation operation : operations.getAll()) {
      operation.trace(trace.shakiradata(), ++stamp);
    }
  }

  @Override
  public String toString() {
    return "ShakiraData{"
        + "operations="
        + operations.operationsInTransactionBundle()
        + ", wcp="
        + wcp
        + ", sha256Blocks="
        + sha256Blocks
        + ", keccak="
        + keccak
        + ", ripemdBlocks="
        + ripemdBlocks
        + ", previousID="
        + previousID
        + '}';
  }
}
