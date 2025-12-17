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

package net.consensys.linea.zktracer.module.txndata;

import java.util.Optional;
import lombok.Getter;
import lombok.Setter;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;

/**
 * This class gathers the block-related information required to trace the {@link TxnData} module.
 */
@Getter
public class BlockSnapshot {
  /** The base fee of this block */
  private final Optional<Wei> baseFee;

  /** The coinbase of this block */
  private final Address coinbaseAddress;

  private final Bytes blockGasLimit;
  @Setter private int nbOfTxsInBlock;

  public BlockSnapshot(ProcessableBlockHeader header) {
    baseFee = header.getBaseFee().map(x -> (Wei) x);
    coinbaseAddress = header.getCoinbase();
    blockGasLimit = Bytes.minimalBytes(header.getGasLimit());
  }
}
