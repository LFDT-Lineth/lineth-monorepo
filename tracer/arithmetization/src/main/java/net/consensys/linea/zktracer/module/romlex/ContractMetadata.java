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

package net.consensys.linea.zktracer.module.romlex;

import net.consensys.linea.zktracer.module.hub.Hub;
import org.hyperledger.besu.datatypes.Address;

/**
 * This class uniquely identify an instance of a contract during the execution of the zkEVM
 *
 * @param address the contract address
 * @param deploymentNumber the current deployment of the contract
 * @param underDeployment whether this contract is being deployed
 */
public record ContractMetadata(
    Address address, int deploymentNumber, boolean underDeployment, int delegationNumber) {
  public static ContractMetadata canonical(Hub hub, Address address) {
    return new ContractMetadata(
        address,
        hub.deploymentNumberOf(address),
        hub.deploymentStatusOf(address),
        hub.delegationNumberOf(address));
  }

  public static ContractMetadata make(
      final Address address, int deploymentNumber, boolean underDeployment, int delegationNumber) {
    return new ContractMetadata(address, deploymentNumber, underDeployment, delegationNumber);
  }
}
