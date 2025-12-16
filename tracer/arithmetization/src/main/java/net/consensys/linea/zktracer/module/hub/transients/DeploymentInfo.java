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

import java.util.HashMap;
import java.util.Map;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;

/** Stores information relative to contract deployment. */
public class DeploymentInfo {
  private final Map<Address, Integer> deploymentNumber = new HashMap<>();
  private final Map<Address, Boolean> deploymentStatus = new HashMap<>();
  private final Map<Address, Bytes> initializationCodes = new HashMap<>();

  /**
   * Returns the deployment number of the given address; sets it to zero if it is the first
   * deployment of this address.
   *
   * @param address the address to get information for
   * @return the deployment number for this address
   */
  public final int deploymentNumber(Address address) {
    return this.getDeploymentNumber(address);
  }

  public void newDeploymentWithExecutionAt(Address address, Bytes bytecode) {
    this.incrementDeploymentNumber(address);
    this.markAsUnderDeployment(address);
    this.setInitializationCode(address, bytecode);
  }

  public void newDeploymentSansExecutionAt(Address address) {
    this.incrementDeploymentNumber(address);
    this.markAsNotUnderDeployment(address);
    this.setInitializationCode(address, Bytes.EMPTY);
  }

  public void deploymentUpdateForSuccessfulSelfDestruct(Address address) {
    this.incrementDeploymentNumber(address);
    this.markAsNotUnderDeployment(address);
    this.setInitializationCode(address, Bytes.EMPTY);
  }

  private int getDeploymentNumber(Address address) {
    return this.deploymentNumber.getOrDefault(address, 0);
  }

  public final boolean getDeploymentStatus(Address address) {
    return this.deploymentStatus.getOrDefault(address, false);
  }

  public final Bytes getInitializationCode(Address address) {
    return this.initializationCodes.get(address);
  }

  private void incrementDeploymentNumber(Address address) {
    int currentDeploymentNumber = getDeploymentNumber(address);
    deploymentNumber.put(address, currentDeploymentNumber + 1);
  }

  private void markAsUnderDeployment(Address address) {
    this.deploymentStatus.put(address, true);
  }

  public final void markAsNotUnderDeployment(Address address) {
    this.deploymentStatus.put(address, false);
  }

  public void setInitializationCode(Address address, Bytes bytecode) {
    this.initializationCodes.put(address, bytecode);
  }
}
