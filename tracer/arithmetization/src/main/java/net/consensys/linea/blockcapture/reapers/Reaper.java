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

package net.consensys.linea.blockcapture.reapers;

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.Trace.*;
import static net.consensys.linea.zktracer.module.hub.section.systemTransaction.EIP2935HistoricalHash.EIP2935_HISTORY_STORAGE_ADDRESS;
import static net.consensys.linea.zktracer.module.hub.section.systemTransaction.EIP4788BeaconBlockRootSection.EIP4788_BEACONROOT_ADDRESS;
import static net.consensys.linea.zktracer.types.PublicInputs.LINEA_BLOB_BASE_FEE_BYTES;

import java.util.*;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.blockcapture.snapshots.AccountSnapshot;
import net.consensys.linea.blockcapture.snapshots.BlockSnapshot;
import net.consensys.linea.blockcapture.snapshots.ConflationSnapshot;
import net.consensys.linea.blockcapture.snapshots.StorageSnapshot;
import net.consensys.linea.blockcapture.snapshots.TransactionResultSnapshot;
import net.consensys.linea.blockcapture.snapshots.TransactionSnapshot;
import net.consensys.linea.zktracer.Fork;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.units.bigints.UInt256;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.evm.log.Log;
import org.hyperledger.besu.evm.worldstate.WorldUpdater;
import org.hyperledger.besu.evm.worldstate.WorldView;
import org.hyperledger.besu.plugin.data.BlockBody;
import org.hyperledger.besu.plugin.data.BlockHeader;

/**
 * The Reaper collect all the information from the state that will be accessed during the execution
 * of a conflation.
 *
 * <p>This data can than be collapsed into a “replay” ({@link ConflationSnapshot}), i.e. the minimal
 * required information to replay a conflation as if it were executed on the blockchain.
 */
@Slf4j
public class Reaper {
  /** Fork provides useful environmental information * */
  private final Fork fork;

  /** Collect storage locations read / written by the entire conflation */
  private final StorageReaper conflationStorage = new StorageReaper();

  /** Collect the account address read / written by the entire conflation */
  private final AddressReaper conflationAddresses = new AddressReaper();

  /** Collection all block hashes read during the conflation * */
  private final Map<Long, Hash> conflationHashes = new HashMap<>();

  /** Collection all blob base fees during the conflation * */
  private final Map<Long, Bytes> conflationBlobBaseFees = new HashMap<>();

  /** Collect the blocks within a conflation */
  private final List<BlockSnapshot> blocks = new ArrayList<>();

  /**
   * Transaction index is used to do determine the current transaction being processed within the
   * current block.
   */
  private int txIndex;

  /** Collect storage locations read / written by the current transaction */
  private StorageReaper txStorage = null;

  /** Collect the account address read / written by the current transaction */
  private AddressReaper txAddresses = null;

  public Reaper(Fork fork) {
    this.fork = fork;
  }

  public void enterBlock(
      final BlockHeader header, final BlockBody body, final Address miningBeneficiary) {
    this.blocks.add(
        BlockSnapshot.of((org.hyperledger.besu.ethereum.core.BlockHeader) header, body));
    this.conflationAddresses.touch(miningBeneficiary);
    txIndex = 0; // reset
    touchedBySystemTransactions(header);
  }

  private void touchedBySystemTransactions(BlockHeader header) {
    // EIP 4788 (CANCUN):
    try {
      conflationAddresses.touch(EIP4788_BEACONROOT_ADDRESS);
      final UInt256 timestamp = UInt256.valueOf(header.getTimestamp());
      final UInt256 timestampIdx = timestamp.mod(HISTORY_BUFFER_LENGTH);
      final UInt256 rootIdx = timestampIdx.add(HISTORY_BUFFER_LENGTH);
      conflationStorage.touch(EIP4788_BEACONROOT_ADDRESS, timestampIdx);
      conflationStorage.touch(EIP4788_BEACONROOT_ADDRESS, rootIdx);
    } catch (Exception e) {
      log.warn(
          "Failed to retrieve EIP4788 infos for block {}, exception caught is: {}",
          header.getNumber(),
          e.getMessage());
    }

    // EIP 2935 (PRAGUE)
    try {
      conflationAddresses.touch(EIP2935_HISTORY_STORAGE_ADDRESS);
      final long blockNumber = header.getNumber();
      final long previousBlockNumber = blockNumber == 0 ? 0 : blockNumber - 1;
      final UInt256 previousBlockNumberMod8191 =
          UInt256.valueOf(previousBlockNumber % HISTORY_SERVE_WINDOW);
      conflationStorage.touch(EIP2935_HISTORY_STORAGE_ADDRESS, previousBlockNumberMod8191);
    } catch (Exception e) {
      log.warn(
          "Failed to retrieve EIP2935 infos for block {}, exception caught is: {}",
          header.getNumber(),
          e.getMessage());
    }
  }

  public void prepareTransaction(Transaction tx) {
    // Configure tx-local reapers
    this.txStorage = new StorageReaper();
    this.txAddresses = new AddressReaper();
    this.touchAddress(tx.getSender());
    tx.getTo().ifPresent(this::touchAddress);
  }

  public void exitTransaction(
      WorldView world,
      boolean status,
      Bytes output,
      List<Log> logs,
      long gasUsed,
      Set<Address> selfDestructs) {
    // Identify relevant transaction snapshot
    TransactionSnapshot txSnapshot = blocks.getLast().txs().get(txIndex);
    // Convert logs into hex strings
    List<String> logStrings = logs.stream().map(l -> l.getData().toHexString()).toList();
    // Convert destructed account addresses into hex strings
    List<String> destructStrings = selfDestructs.stream().map(Address::toHexString).toList();
    // Collapse accounts
    final List<AccountSnapshot> accounts = this.txAddresses.collapse(world);
    // Collapse storage
    final List<StorageSnapshot> storage = txStorage.collapse(world);
    // Construct the result snapshot
    TransactionResultSnapshot resultSnapshot =
        new TransactionResultSnapshot(
            status, output.toHexString(), logStrings, gasUsed, accounts, storage, destructStrings);
    // Record the outcome
    txSnapshot.setTransactionResult(resultSnapshot);
    // Reset for next transaction
    txIndex++;
    this.txStorage = null;
    this.txAddresses = null;
  }

  public void touchAddress(final Address address) {
    this.conflationAddresses.touch(address);
    this.txAddresses.touch(address);
  }

  public void touchStorage(final Address address, final UInt256 key) {
    this.conflationStorage.touch(address, key);
    this.txStorage.touch(address, key);
  }

  public void touchBlockHash(final long blockNumber, Hash blockHash) {
    if (!conflationHashes.containsKey(blockNumber) || conflationHashes.get(blockNumber).isEmpty()) {
      conflationHashes.put(blockNumber, blockHash);
    } else {
      checkArgument(
          conflationHashes.get(blockNumber).equals(blockHash),
          "Block hash mismatch for block number %s: existing %s, new %s",
          blockNumber,
          conflationHashes.get(blockNumber),
          blockHash);
    }
  }

  public void touchBlobBaseFee(final long blockNumber, Bytes blobBaseFee) {
    checkArgument(
        conflationBlobBaseFees.getOrDefault(blockNumber, blobBaseFee).equals(blobBaseFee),
        "BLOBBASEFEE must be constant along a block");
    conflationBlobBaseFees.put(blockNumber, blobBaseFee);
  }

  /**
   * Uniquify and solidify the accumulated data, then return a {@link ConflationSnapshot}, which
   * contains the smallest dataset required to exactly replay the conflation within a test framework
   * without requiring access to the whole state.
   *
   * @param world the state before the conflation execution
   * @return a minimal set of information required to replay the conflation within a test framework
   */
  public ConflationSnapshot collapse(final WorldUpdater world) {
    // Collapse accounts
    final List<AccountSnapshot> accounts = conflationAddresses.collapse(world);
    // Collapse storage
    final List<StorageSnapshot> storage = conflationStorage.collapse(world);
    // Collapse BlobBaseFee: if a blobBaseFee of a block is not known, copy/paste the one of the
    // previous block. For the first block of the conflation, if not known, write the LINEA default
    // value.
    Bytes previousBlobBaseFee =
        conflationBlobBaseFees.getOrDefault(
            blocks.getFirst().header().number(), LINEA_BLOB_BASE_FEE_BYTES);
    for (BlockSnapshot block : blocks) {
      if (!conflationBlobBaseFees.containsKey(block.header().number())) {
        conflationBlobBaseFees.put(block.header().number(), previousBlobBaseFee);
      } else {
        previousBlobBaseFee = conflationBlobBaseFees.get(block.header().number());
      }
    }
    // Done
    return ConflationSnapshot.from(
        fork.name(), blocks, accounts, storage, conflationHashes, conflationBlobBaseFees);
  }
}
