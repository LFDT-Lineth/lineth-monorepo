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

import java.util.List;
import org.hyperledger.besu.ethereum.core.BlockHeader;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.plugin.data.BlockBody;

public record BlockSnapshot(BlockHeaderSnapshot header, List<TransactionSnapshot> txs) {
  public static BlockSnapshot of(final BlockHeader header, final BlockBody body) {
    return new BlockSnapshot(
        BlockHeaderSnapshot.from(header),
        body.getTransactions().stream().map(t -> TransactionSnapshot.of((Transaction) t)).toList());
  }
}
