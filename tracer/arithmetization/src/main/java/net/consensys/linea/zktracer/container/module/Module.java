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

package net.consensys.linea.zktracer.container.module;

import java.util.List;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.worldstate.WorldView;
import org.hyperledger.besu.plugin.data.BlockBody;
import org.hyperledger.besu.plugin.data.BlockHeader;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;

public interface Module {
  ModuleName moduleKey();

  default void traceStartConflation(final long blockCount) {}

  default void traceEndConflation(final WorldView state) {
    this.commitTransactionBundle();
  }

  default void traceStartBlock(
      WorldView world,
      final ProcessableBlockHeader processableBlockHeader,
      final Address miningBeneficiary) {}

  default void traceEndBlock(final BlockHeader blockHeader, final BlockBody blockBody) {}

  default void traceStartTx(
      WorldView worldView, TransactionProcessingMetadata transactionProcessingMetadata) {}

  default void traceEndTx(TransactionProcessingMetadata tx) {}

  default void traceContextEnter(MessageFrame frame) {}

  default void traceContextExit(MessageFrame frame) {}

  /**
   * Called when a bundle of transaction execution is cancelled; should revert the state of the
   * module.
   */
  void popTransactionBundle();

  /**
   * Called when a bundle of transactions is committed. Those transactions can't be cancelled
   * afterward.
   */
  void commitTransactionBundle();

  /**
   * Report the number of lines in the trace of a given module as it stands. Observe that this does
   * not include spillage information and, hence, is a lower bound on the number of lines in the
   * final trace.
   *
   * @return
   */
  int lineCount();

  /**
   * Report spillage required for a given module. Spillage represents additional lines that will be
   * added on top of the lineCount to the final trace for the module question. As such, this needs
   * to be incorporated when determine the final line count for a module.
   *
   * @return
   */
  int spillage(Trace trace);

  List<Trace.ColumnHeader> columnHeaders(Trace trace);

  default void commit(Trace trace) {
    throw new UnsupportedOperationException("Can't trace module " + moduleKey());
  }
}
