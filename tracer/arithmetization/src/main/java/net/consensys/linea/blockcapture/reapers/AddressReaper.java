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

import java.util.Arrays;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import net.consensys.linea.blockcapture.snapshots.AccountSnapshot;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.worldstate.WorldView;

public class AddressReaper {
  private final Set<Address> touchedAccounts = new HashSet<>();

  public void touch(final Address... addresses) {
    this.touchedAccounts.addAll(Arrays.asList(addresses));
  }

  /**
   * Collapse recorded set of touched accounts down into a list of account snapshots using a given
   * world-state to determine what their state is.
   *
   * @param world The world state to use for extracting current account balances, etc.
   * @return List of account snapshots
   */
  public List<AccountSnapshot> collapse(final WorldView world) {
    return this.touchedAccounts.stream()
        .flatMap(a -> AccountSnapshot.from(a, world).stream())
        .toList();
  }
}
