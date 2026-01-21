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

import java.util.Comparator;

class RomOperationComparator implements Comparator<RomOperation> {
  public int compare(RomOperation chunk1, RomOperation chunk2) {
    // First sort by Address
    final int addressComparison =
        chunk1.metadata().address().compareTo(chunk2.metadata().address());
    if (addressComparison != 0) {
      return addressComparison;
    } else {
      // Second, sort by Deployment Number
      int deploymentNumberComparison =
          chunk1.metadata().deploymentNumber() - chunk2.metadata().deploymentNumber();
      if (deploymentNumberComparison != 0) {
        return deploymentNumberComparison;
      } else {
        // Third sort by Deployment Status (true greater)
        if (chunk1.metadata().underDeployment() == chunk2.metadata().underDeployment()) {
          // Fourth sort by delegation number
          return chunk1.metadata().delegationNumber() - chunk2.metadata().delegationNumber();
        } else {
          return chunk1.metadata().underDeployment() ? -1 : 1;
        }
      }
    }
  }
}
