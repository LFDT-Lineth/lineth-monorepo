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

package net.consensys.linea.zktracer.module.limits;

import static net.consensys.linea.zktracer.module.ModuleName.BLOCK_L1_SIZE;

import java.util.List;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.IncrementingModule;
import net.consensys.linea.zktracer.container.module.Module;
import net.consensys.linea.zktracer.container.stacked.CountOnlyOperation;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Log;
import org.hyperledger.besu.datatypes.LogTopic;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.evm.worldstate.WorldView;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;

@Accessors(fluent = true)
@Getter
@RequiredArgsConstructor
public class L1BlockSize implements Module {

  private final BlockTransactions blockTransactions;
  private final Keccak keccak;
  private final IncrementingModule l2l1Logs;
  private final Address l2l1Address;
  private final LogTopic l2l1Topic;

  private final short TIMESTAMP_BYTESIZE = 32 / 8;
  private final short NB_TX_IN_BLOCK_BYTESIZE = 16 / 8;

  /** The byte size of the RLP-encoded transaction of the conflation */
  private final CountOnlyOperation sizesRlpEncodedTxs = new CountOnlyOperation();

  /** The byte size of the L2->L1 logs messages of the conflation */
  private final CountOnlyOperation l2l1LogSizes = new CountOnlyOperation();

  /** The number of block of the current conflation */
  private short nbBlock = 0;

  @Override
  public ModuleName moduleKey() {
    return BLOCK_L1_SIZE;
  }

  @Override
  public void commitTransactionBundle() {
    sizesRlpEncodedTxs.commitTransactionBundle();
    l2l1LogSizes.commitTransactionBundle();
  }

  @Override
  public void popTransactionBundle() {
    sizesRlpEncodedTxs.popTransactionBundle();
    l2l1LogSizes.popTransactionBundle();
  }

  @Override
  public int lineCount() {

    return sizesRlpEncodedTxs.lineCount()

        // Calculates the data size related to the abi encoding of the list of the
        // from addresses. The field is a simple array of bytes20.
        + blockTransactions.lineCount() * Address.SIZE

        // Calculates the data size related to the block
        // TODO: remove the Hash.EMPTY workaround
        + nbBlock * (TIMESTAMP_BYTESIZE + Hash.EMPTY.getBytes().size() + NB_TX_IN_BLOCK_BYTESIZE);
  }

  @Override
  public int spillage(Trace trace) {
    return 0;
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    throw new IllegalStateException("non-tracing module");
  }

  @Override
  public void traceEndTx(TransactionProcessingMetadata tx) {
    traceEndTx(tx.getBesuTransaction(), tx.getLogs());
  }

  public void traceEndTx(Transaction tx, List<Log> logs) {
    for (Log log : logs) {
      if (isL2L1Log(log)) {
        l2l1LogSizes.add(log.getData().size());
        // The L2L1Logs module counts only the number of L2->L1 logs
        l2l1Logs.updateTally(1);
      }
    }

    // This calculates the data size related to the transaction field of the
    // data sent on L1. This field is a double array of byte. Each subarray
    // corresponds to an RLP encoded transaction. The abi encoding incurs an
    // overhead for each transaction (32 bytes for an offset, and 32 bytes for
    // to encode the length of each sub bytes array). This overhead is also
    // incurred by the top-level array, hence the +1.
    final int txDataSize = tx.encoded().size();
    sizesRlpEncodedTxs.add(txDataSize);
    // Counts the number of Keccak from tx RLPs, used both for both the signature verification and
    // the public input computation.
    keccak.updateTally(txDataSize);
    // Counts the number of Keccak from tx RLPs preimage (RLP of the transaction wo the signature)
    final int txPreimageByteSize = tx.encodedPreimage().size();
    keccak.updateTally(txPreimageByteSize);
  }

  @Override
  public void traceStartBlock(
      WorldView world,
      final ProcessableBlockHeader processableBlockHeader,
      final Address miningBeneficiary) {
    nbBlock++;
  }

  private boolean isL2L1Log(Log log) {
    return log.getLogger().getBytes().equals(l2l1Address.getBytes())
        && !log.getTopics().isEmpty()
        && log.getTopics().getFirst().getBytes().equals(l2l1Topic.getBytes());
  }
}
