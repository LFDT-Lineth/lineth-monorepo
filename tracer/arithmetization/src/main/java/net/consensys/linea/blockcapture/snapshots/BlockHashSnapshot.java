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

import org.hyperledger.besu.datatypes.Hash;

/**
 * Responsible for storing the block hash of a given block.
 *
 * @param blockNumber
 * @param blockHash
 */
public record BlockHashSnapshot(long blockNumber, String blockHash) {
  public static BlockHashSnapshot of(final long blockNumber, final Hash blockHash) {
    return new BlockHashSnapshot(blockNumber, blockHash.getBytes().toHexString());
  }
}
