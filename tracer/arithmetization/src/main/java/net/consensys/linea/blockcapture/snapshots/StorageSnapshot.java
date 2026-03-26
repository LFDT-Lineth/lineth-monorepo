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

import java.util.Optional;
import org.apache.tuweni.units.bigints.UInt256;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.account.Account;
import org.hyperledger.besu.evm.worldstate.WorldView;

public record StorageSnapshot(String address, String key, String value) {
  public static Optional<StorageSnapshot> from(
      Address address, UInt256 key, final WorldView world) {
    // Lookup account that was touched
    Account account = world.get(address);
    // Check account *really* exists from a storage perspective.  This is important to distinguish
    // for accounts which are created *during* the conflation.  Such accounts may have technically
    // existed before the conflation (e.g. they had a non-zero balance) but could still have been
    // "created" during the conflation.  In such case, this snapshot would be simply assigning 0x0
    // to the given storage locations.  However, we don't want to create a storage snapshot in such
    // cases, as this then leads to a CREATE[2] failure when executing the conflation.
    if (accountExists(account)) {
      // Accounts exists, so create snapshot.
      return Optional.of(
          new StorageSnapshot(
              address.getBytes().toHexString(),
              key.toHexString(),
              account.getStorageValue(key).toHexString()));
    } else {
      return Optional.empty();
    }
  }

  private static boolean accountExists(final Account account) {
    // The account exists if it has sent a transaction
    // or already has its code initialized.
    return account != null
        && (account.getNonce() != 0 || !account.getCode().isEmpty() || !account.isStorageEmpty());
  }
}
