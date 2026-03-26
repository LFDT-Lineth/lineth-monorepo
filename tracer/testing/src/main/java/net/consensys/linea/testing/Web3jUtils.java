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

import org.web3j.protocol.core.methods.response.Log;

public class Web3jUtils {
  /**
   * Creates a web3j Log object from the Besu Log object. Web3j Log object can use used which web3j
   * APIs to parse events in the log.
   *
   * @param log The besu log object
   * @return The web3j log object
   */
  public static Log fromBesuLog(org.hyperledger.besu.datatypes.Log log) {
    return new Log(
        false,
        "",
        "",
        "",
        "",
        "",
        log.getLogger().getBytes().toHexString(),
        log.getData().toHexString(),
        "",
        log.getTopics().stream().map(it -> it.getBytes().toHexString()).toList());
  }
}
