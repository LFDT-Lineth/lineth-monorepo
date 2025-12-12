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

package net.consensys.linea.zktracer.module.limits;

import static net.consensys.linea.zktracer.module.ModuleName.BLOCK_TRANSACTIONS;

import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.container.module.IncrementingModule;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.hyperledger.besu.evm.worldstate.WorldView;

@Getter
@Accessors(fluent = true)
public class BlockTransactions extends IncrementingModule {

  public BlockTransactions() {
    super(BLOCK_TRANSACTIONS);
  }

  @Override
  public void traceStartTx(
      WorldView worldView, TransactionProcessingMetadata transactionProcessingMetadata) {
    updateTally(1);
  }
}
