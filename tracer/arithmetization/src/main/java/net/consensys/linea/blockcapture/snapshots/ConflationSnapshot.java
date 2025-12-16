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

package net.consensys.linea.blockcapture.snapshots;

import static net.consensys.linea.zktracer.Trace.BLOCKHASH_MAX_HISTORY;
import static net.consensys.linea.zktracer.types.PublicInputs.getDefaultBlobBaseFees;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.evm.blockhash.BlockHashLookup;
import org.hyperledger.besu.evm.frame.MessageFrame;

/**
 * Contain the minimal set of information to replay a conflation as a unit test without requiring
 * access to the whole state.
 *
 * @param blocks the blocks within the conflation
 * @param accounts the accounts whose state will be read during the conflation execution
 * @param storage storage cells that will be accessed during the conflation execution
 */
public record ConflationSnapshot(
    String fork,
    List<BlockSnapshot> blocks,
    List<AccountSnapshot> accounts,
    List<StorageSnapshot> storage,
    List<BlockHashSnapshot> blockHashes,
    Map<Long, byte[]> blobBaseFees) {

  public static ConflationSnapshot from(
      String fork,
      List<BlockSnapshot> blocks,
      List<AccountSnapshot> accounts,
      List<StorageSnapshot> storage,
      Map<Long, Hash> blockHashes,
      Map<Long, Bytes> blobBaseFees) {
    final ArrayList<BlockHashSnapshot> blockHashSnapshots = new ArrayList<>();
    final Map<Long, byte[]> bbFees = new HashMap<>();
    //
    for (Map.Entry<Long, Hash> e : blockHashes.entrySet()) {
      String h = e.getValue().toHexString();
      blockHashSnapshots.add(new BlockHashSnapshot(e.getKey(), h));
    }
    // Encoding blob base fee
    for (Map.Entry<Long, Bytes> e : blobBaseFees.entrySet()) {
      bbFees.put(e.getKey(), e.getValue().toArray());
    }
    //
    return new ConflationSnapshot(fork, blocks, accounts, storage, blockHashSnapshots, bbFees);
  }

  public long firstBlockNumber() {
    if (blocks.isEmpty()) {
      return Long.MAX_VALUE;
    }
    // Extract number of first block
    return blocks.getFirst().header().number();
  }

  public long lastBlockNumber() {
    if (blocks.isEmpty()) {
      return Long.MAX_VALUE;
    }
    // Extract number of last block
    return blocks.getLast().header().number();
  }

  public Map<Long, Hash> historicalBlockHashes() {
    final long firstBlockToRetrieve = Math.max(0, firstBlockNumber() - BLOCKHASH_MAX_HISTORY);
    final long lastBlockToRetrieve = Math.max(0, lastBlockNumber() - 1);
    final HashMap<Long, Hash> hashes = new HashMap<>();
    // Initialise map of historical hashes
    for (BlockHashSnapshot blkHash : blockHashes) {
      long key = blkHash.blockNumber();
      if (key >= firstBlockToRetrieve && key <= lastBlockToRetrieve) {
        hashes.put(key, Hash.fromHexString(blkHash.blockHash()));
      }
    }
    // Done
    return hashes;
  }

  public Map<Long, Bytes> blobBaseFeesOrDefault() {
    if (blobBaseFees == null) {
      return getDefaultBlobBaseFees(firstBlockNumber(), lastBlockNumber());
    }
    // Decode
    Map<Long, Bytes> bbFees = new HashMap<>();
    //
    for (Map.Entry<Long, byte[]> e : blobBaseFees.entrySet()) {
      bbFees.put(e.getKey(), Bytes.of(e.getValue()));
    }
    //
    return bbFees;
  }

  /**
   * Construct a block hash map for any block hashes embedded in this conflation.
   *
   * @return
   */
  public BlockHashLookup toBlockHashLookup() {
    final BlockHashMap map = new BlockHashMap();
    // Initialise block hashes.  This can be null for replays which pre-date support for block hash
    // capture and, hence, we must support this case (at least for now).
    if (blockHashes() != null) {
      // Initialise block hash cache
      for (BlockHashSnapshot h : blockHashes) {
        Hash blockHash = Hash.fromHexString(h.blockHash());
        map.blockHashCache.put(h.blockNumber(), blockHash);
      }
    }
    // Done
    return map;
  }

  private static class BlockHashMap implements BlockHashLookup {
    /**
     * The hash cache simply stores known hashes for blocks. All the needed hashes for execution
     * should have been captured by the BlockCapturer and stored in the conflation.
     */
    private final Map<Long, Hash> blockHashCache = new HashMap<>();

    @Override
    public Hash apply(MessageFrame frame, Long blockNumber) {
      // Sanity check we found the hash
      if (!this.blockHashCache.containsKey(blockNumber)) {
        // Missing for some reason
        throw new IllegalArgumentException("missing hash of block " + blockNumber);
      }
      // Yes, we have it.
      return this.blockHashCache.get(blockNumber);
    }
  }
}
