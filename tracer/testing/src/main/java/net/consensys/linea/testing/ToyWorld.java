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

package net.consensys.linea.testing;

import java.util.Collection;
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import lombok.Builder;
import lombok.Getter;
import lombok.Singular;
import net.consensys.linea.blockcapture.snapshots.AccountSnapshot;
import net.consensys.linea.blockcapture.snapshots.BlockHashSnapshot;
import net.consensys.linea.blockcapture.snapshots.ConflationSnapshot;
import net.consensys.linea.blockcapture.snapshots.StorageSnapshot;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.units.bigints.UInt256;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.evm.account.Account;
import org.hyperledger.besu.evm.account.MutableAccount;
import org.hyperledger.besu.evm.internal.Words;
import org.hyperledger.besu.evm.worldstate.WorldUpdater;

public class ToyWorld implements WorldUpdater {
  /**
   * The hash cache simply stores known hashes for blocks. All the needed hashes for execution
   * should have been captured by the BlockCapturer and stored in the conflation.
   */
  private final Map<Long, Hash> blockHashCache = new HashMap<>();

  private final ToyWorld parent;
  @Getter private Map<Address, ToyAccount> addressAccountMap;

  private ToyWorld() {
    this(null);
  }

  private ToyWorld(final ToyWorld parent) {
    this(parent, Collections.emptyList());
  }

  @Builder
  private ToyWorld(final ToyWorld parent, @Singular final List<ToyAccount> accounts) {
    this.parent = parent;
    this.addressAccountMap = new HashMap<>();
    // Initialise the account map
    for (ToyAccount account : accounts) {
      addressAccountMap.put(account.getAddress(), account);
    }
  }

  public static ToyWorld empty() {
    return builder().build();
  }

  public static ToyWorld of(final ConflationSnapshot conflation) {
    final ToyWorld world = new ToyWorld();
    initWorldUpdater(world, conflation);
    // Initialise block hashes.  This can be null for replays which pre-date support for block hash
    // capture and,
    // hence, we must support this case (at least for now).
    if (conflation.blockHashes() != null) {
      for (BlockHashSnapshot h : conflation.blockHashes()) {
        Hash blockHash = Hash.fromHexString(h.blockHash());
        world.blockHashCache.put(h.blockNumber(), blockHash);
      }
    }
    // Done
    return world;
  }

  @Override
  public WorldUpdater updater() {
    return new ToyWorld(this);
  }

  @Override
  public Account get(final Address address) {
    if (addressAccountMap.containsKey(address)) {
      return addressAccountMap.get(address);
    } else if (parent != null) {
      return parent.get(address);
    } else {
      return null;
    }
  }

  @Override
  public MutableAccount createAccount(final Address address, final long nonce, final Wei balance) {
    return createAccount(null, address, nonce, balance, Bytes.EMPTY);
  }

  public MutableAccount createAccount(
      final Account parentAccount,
      final Address address,
      final long nonce,
      final Wei balance,
      final Bytes code) {

    ToyAccount account =
        ToyAccount.builder()
            .parent(parentAccount)
            .code(code)
            .address(address)
            .nonce(nonce)
            .balance(balance)
            .build();

    addressAccountMap.put(address, account);

    return account;
  }

  @Override
  public MutableAccount getAccount(final Address address) {
    if (addressAccountMap.containsKey(address)) {
      return addressAccountMap.get(address);
    } else if (parent != null) {
      Account parentAccount = parent.get(address);
      if (parentAccount == null) {
        return null;
      } else {
        return createAccount(
            parentAccount,
            parentAccount.getAddress(),
            parentAccount.getNonce(),
            parentAccount.getBalance(),
            parentAccount.getCode());
      }
    } else {
      return null;
    }
  }

  @Override
  public void deleteAccount(final Address address) {
    addressAccountMap.put(address, null);
  }

  @Override
  public Collection<? extends Account> getTouchedAccounts() {
    return addressAccountMap.values();
  }

  @Override
  public Collection<Address> getDeletedAccountAddresses() {
    return addressAccountMap.entrySet().stream()
        .filter(e -> e.getValue() == null)
        .map(Map.Entry::getKey)
        .toList();
  }

  @Override
  public void revert() {
    addressAccountMap = new HashMap<>();
  }

  @Override
  public void commit() {
    addressAccountMap.forEach(
        (address, account) -> {
          if (account == null || !account.updateParent()) {
            parent.addressAccountMap.put(address, account);
          }
        });
  }

  @Override
  public Optional<WorldUpdater> parentUpdater() {
    return Optional.ofNullable(parent);
  }

  /**
   * Obtain the block hash for a given block.
   *
   * @param blockNumber The block number for which to obtain the hash.
   * @return Hash The hash of the block.
   */
  public Hash blockHash(long blockNumber) {
    // Sanity check we found the hash
    if (!this.blockHashCache.containsKey(blockNumber)) {
      throw new IllegalArgumentException("missing hash of block " + blockNumber);
    }
    // Yes, we have it.
    return this.blockHashCache.get(blockNumber);
  }

  /**
   * Initialise a world updater given a conflation. Observe this can be applied to any WorldUpdater,
   * such as SimpleWorld.
   *
   * @param world The world to be initialised.
   * @param conflation The conflation from which to initialise.
   */
  private static void initWorldUpdater(WorldUpdater world, final ConflationSnapshot conflation) {
    for (AccountSnapshot account : conflation.accounts()) {
      // Construct contract address
      Address addr = Address.fromHexString(account.address());
      // Create account
      MutableAccount acc =
          world.createAccount(
              Words.toAddress(addr.getBytes()),
              account.nonce(),
              Wei.fromHexString(account.balance()));
      // Update code
      acc.setCode(Bytes.fromHexString(account.code()));
    }
    // Initialise storage
    for (StorageSnapshot s : conflation.storage()) {
      world
          .getAccount(Words.toAddress(Bytes.fromHexString(s.address())))
          .setStorageValue(UInt256.fromHexString(s.key()), UInt256.fromHexString(s.value()));
    }
  }
}
