/*
 * Copyright contributors to Hyperledger Besu
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
 *
 */

package net.consensys.linea.testing;

import static net.consensys.linea.zktracer.Trace.EIP_7702_DELEGATION_INDICATOR;

import com.google.common.base.Suppliers;
import java.util.HashMap;
import java.util.Map;
import java.util.NavigableMap;
import java.util.function.Supplier;
import lombok.Builder;
import lombok.Getter;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.apache.tuweni.units.bigints.UInt256;
import org.hyperledger.besu.config.GenesisAccount;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECPPrivateKey;
import org.hyperledger.besu.crypto.SECPPublicKey;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.referencetests.ReferenceTestWorldState;
import org.hyperledger.besu.evm.ModificationNotAllowedException;
import org.hyperledger.besu.evm.account.Account;
import org.hyperledger.besu.evm.account.AccountStorageEntry;
import org.hyperledger.besu.evm.account.MutableAccount;

@Builder
public class ToyAccount implements MutableAccount {
  private final Account parent;

  @Builder.Default private boolean mutable = true;

  @Getter private Address address;
  private final Supplier<Hash> addressHash = Suppliers.memoize(() -> address.addressHash());
  @Getter private long nonce;
  @Getter @Builder.Default private Wei balance = Wei.ZERO;
  @Getter @Builder.Default private Bytes code = Bytes.EMPTY;
  private Supplier<Hash> codeHash;
  @Builder.Default final Map<UInt256, UInt256> storage = new HashMap<>();
  @Getter final KeyPair keyPair;

  public ToyAccount deepCopy() {
    ToyAccount copy =
        ToyAccount.builder()
            .mutable(this.mutable)
            .address(Address.wrap(this.address.copy()))
            .nonce(this.nonce)
            .balance(Wei.of(this.balance.toLong()))
            .code(Bytes.wrap(this.code.toArray()))
            .keyPair(
                new KeyPair(
                    SECPPrivateKey.create(
                        Bytes32.wrap(this.keyPair.getPrivateKey().getEncoded()),
                        this.keyPair.getPrivateKey().getAlgorithm()),
                    SECPPublicKey.create(
                        Bytes.wrap(this.keyPair.getPublicKey().getEncoded()),
                        this.keyPair.getPublicKey().getAlgorithm())))
            .build();
    for (Map.Entry<UInt256, UInt256> e : this.storage.entrySet()) {
      copy.storage.put(
          UInt256.fromBytes(e.getKey().toBytes()), UInt256.fromBytes(e.getValue().toBytes()));
    }
    return copy;
  }

  @Override
  public boolean isStorageEmpty() {
    return storage.isEmpty();
  }

  @Override
  public Hash getAddressHash() {
    return addressHash.get();
  }

  @Override
  public Hash getCodeHash() {
    return Suppliers.memoize(() -> Hash.hash(code)).get();
  }

  @Override
  public UInt256 getStorageValue(final UInt256 key) {
    if (storage.containsKey(key)) {
      return storage.get(key);
    } else if (parent != null) {
      return getOriginalStorageValue(key);
    }

    return UInt256.ZERO;
  }

  @Override
  public UInt256 getOriginalStorageValue(final UInt256 key) {
    if (parent != null) {
      return parent.getStorageValue(key);
    } else {
      return getStorageValue(key);
    }
  }

  @Override
  public NavigableMap<Bytes32, AccountStorageEntry> storageEntriesFrom(
      final Bytes32 startKeyHash, final int limit) {
    throw new UnsupportedOperationException("Storage iteration not supported in toy evm");
  }

  @Override
  public void setNonce(final long value) {
    if (!mutable) {
      throw new ModificationNotAllowedException();
    }
    nonce = value;
  }

  @Override
  public void setBalance(final Wei value) {
    if (!mutable) {
      throw new ModificationNotAllowedException();
    }
    balance = value;
  }

  @Override
  public void setCode(final Bytes code) {
    if (!mutable) {
      throw new ModificationNotAllowedException();
    }
    this.code = code;
    this.codeHash = Suppliers.memoize(() -> this.code == null ? Hash.EMPTY : Hash.hash(this.code));
  }

  @Override
  public void setStorageValue(final UInt256 key, final UInt256 value) {
    if (!mutable) {
      throw new ModificationNotAllowedException();
    }
    storage.put(key, value);
  }

  @Override
  public void clearStorage() {
    if (!mutable) {
      throw new ModificationNotAllowedException();
    }
    storage.clear();
  }

  @Override
  public Map<UInt256, UInt256> getUpdatedStorage() {
    return this.storage;
  }

  @Override
  public void becomeImmutable() {
    mutable = false;
  }

  /**
   * Push changes into the parent account, if one exists
   *
   * @return true if a parent account was updated, false if not (this indicates the account should
   *     be inserted into the parent contact).
   */
  public boolean updateParent() {
    if (parent instanceof ToyAccount parentAsToyAccount) {
      parentAsToyAccount.balance = balance;
      parentAsToyAccount.nonce = nonce;
      parentAsToyAccount.storage.putAll(storage);
      return true;
    } else {
      return false;
    }
  }

  public GenesisAccount toGenesisAccount() {
    return new GenesisAccount(
        address,
        nonce,
        balance,
        code,
        storage,
        keyPair == null ? null : Bytes32.wrap(keyPair.getPrivateKey().getEncoded()));
  }

  public ReferenceTestWorldState.AccountMock toAccountMock() {
    Map<String, String> accountMockStorage = new HashMap<>();
    for (Map.Entry<UInt256, UInt256> e : storage.entrySet()) {
      accountMockStorage.put(e.getKey().toHexString(), e.getValue().toHexString());
    }
    return new ReferenceTestWorldState.AccountMock(
        Long.toHexString(nonce), balance.toHexString(), accountMockStorage, code.toHexString());
  }

  // EIP-7702
  public ToyAccount delegateTo(Address address) {
    this.code =
        Bytes.fromHexString(
            Integer.toHexString(EIP_7702_DELEGATION_INDICATOR)
                + address.toHexString().substring(2));
    return this;
  }

  public ToyAccount delegateTo(ToyAccount account) {
    return delegateTo(account.getAddress());
  }
}
