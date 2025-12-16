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

package net.consensys.linea.zktracer.types;

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.Trace.BLOCKHASH_MAX_HISTORY;
import static net.consensys.linea.zktracer.Trace.LINEA_BLOB_BASE_FEE;

import java.util.HashMap;
import java.util.Map;
import lombok.experimental.Accessors;
import lombok.extern.slf4j.Slf4j;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.plugin.data.BlockContext;
import org.hyperledger.besu.plugin.data.BlockHeader;
import org.hyperledger.besu.plugin.services.BlockchainService;

@Slf4j
@Accessors(fluent = true)
public record PublicInputs(Map<Long, Hash> historicalBlockhashes, Map<Long, Bytes> blobBaseFees) {

  public static final Bytes LINEA_BLOB_BASE_FEE_BYTES = Bytes.minimalBytes(LINEA_BLOB_BASE_FEE);

  public static PublicInputs emptyPublicInputs() {
    log.info("[ZkTracer] No public input provided, assuming line counting only or testing.");
    return new PublicInputs(new HashMap<>(), new HashMap<>());
  }

  public static PublicInputs emptyHistoricalBlockhashes(Map<Long, Bytes> blobBaseFees) {
    return new PublicInputs(new HashMap<>(), blobBaseFees);
  }

  public static PublicInputs defaultEmptyHistoricalBlockhashes(
      long firstBlockNumber, long lastBlockNumber) {
    return new PublicInputs(
        new HashMap<>(), getDefaultBlobBaseFees(firstBlockNumber, lastBlockNumber));
  }

  public boolean allPublicInputsKnown() {
    if (historicalBlockhashes == null || blobBaseFees == null) {
      return false;
    }
    return historicalBlockhashes.size() >= BLOCKHASH_MAX_HISTORY && !blobBaseFees.isEmpty();
  }

  public boolean missingSomeInfos() {
    return !allPublicInputsKnown();
  }

  // Some methods to retrieve public inputs information from BlockchainService
  public static PublicInputs generatePublicInputs(
      BlockchainService blockchain,
      long firstBlockNumberOfConflation,
      long lastBlockNumberOfConflation) {
    return new PublicInputs(
        retrieveHistoricalBlockHashes(
            blockchain, firstBlockNumberOfConflation, lastBlockNumberOfConflation),
        getBlobBaseFees(blockchain, firstBlockNumberOfConflation, lastBlockNumberOfConflation));
  }

  public static Map<Long, Hash> retrieveHistoricalBlockHashes(
      BlockchainService blockchain,
      long firstBlockNumberOfConflation,
      long lastBlockNumberOfConflation) {

    final Map<Long, Hash> historicalBlockHashes =
        new HashMap<>(
            (int)
                (BLOCKHASH_MAX_HISTORY
                    + lastBlockNumberOfConflation
                    - firstBlockNumberOfConflation));

    final long firstBlockToRetrieve =
        Math.max(firstBlockNumberOfConflation - BLOCKHASH_MAX_HISTORY, 0);
    final long lastBlockToRetrieve =
        lastBlockNumberOfConflation == 0 ? 0 : lastBlockNumberOfConflation - 1;

    for (long blockNumber = lastBlockToRetrieve;
        blockNumber >= firstBlockToRetrieve;
        blockNumber--) {
      final long blockNumberAttempt = blockNumber;
      final Hash hash =
          blockchain
              .getBlockByNumber(blockNumberAttempt)
              .orElseThrow(
                  () ->
                      new IllegalArgumentException(
                          "When retrieving historical blockhashes, block number: "
                              + blockNumberAttempt
                              + " was not found"))
              .getBlockHeader()
              .getBlockHash();
      historicalBlockHashes.put(blockNumber, hash);
    }
    return historicalBlockHashes;
  }

  public static Map<Long, Bytes> getBlobBaseFees(
      BlockchainService blockchainService, long fromBlock, long toBlock) {
    final Map<Long, Bytes> blobBaseFees = new HashMap<>((int) (toBlock - fromBlock + 1));
    for (long l = fromBlock; l <= toBlock; l++) {
      final long blockNumber = l;
      final BlockContext block =
          blockchainService
              .getBlockByNumber(blockNumber)
              .orElseThrow(() -> new IllegalArgumentException("Block not found: " + blockNumber));
      final BlockHeader header = block.getBlockHeader();
      final Bytes blobBaseFee = blockchainService.getBlobGasPrice(header);
      checkArgument(blobBaseFee != null, "Blob base fee is null for block number " + blockNumber);
      blobBaseFees.put(blockNumber, blobBaseFee);
    }
    return blobBaseFees;
  }

  public static Map<Long, Bytes> getDefaultBlobBaseFees(long fromBlock, long toBlock) {
    final Map<Long, Bytes> blobBaseFees = new HashMap<>((int) (toBlock - fromBlock + 1));
    for (long blockNumber = fromBlock; blockNumber <= toBlock; blockNumber++) {
      // Just put the linea blob base fee constant
      blobBaseFees.put(blockNumber, LINEA_BLOB_BASE_FEE_BYTES);
    }
    return blobBaseFees;
  }
}
