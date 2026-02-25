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

package net.consensys.linea.zktracer.module.hub.transients;

import java.util.*;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.container.stacked.StackedSet;
import net.consensys.linea.zktracer.runtime.LogData;
import org.hyperledger.besu.datatypes.Address;

/** Stores data relative to the conflation. */
@Accessors(fluent = true)
@Getter
public class Conflation {
  private final DeploymentInfo deploymentInfo = new DeploymentInfo();
  private final List<LogData> logs = new ArrayList<>(100);
  private final StackedSet<StackHeightCheck> stackHeightChecksForStackUnderflows =
      new StackedSet<>(256, 32);
  private final StackedSet<StackHeightCheck> stackHeightChecksForStackOverflows =
      new StackedSet<>(256, 32);
  private final Map<Address, Integer> delegationNumber = new HashMap<>();

  public int getDelegationNumber(Address address) {
    return delegationNumber.getOrDefault(address, 0);
  }

  public void incrementDelegationNumber(Address address) {
    delegationNumber.put(address, getDelegationNumber(address) + 1);
  }
}
