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
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.LogsBloomFilter;
import org.hyperledger.besu.datatypes.Quantity;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.BlockHeader;
import org.hyperledger.besu.ethereum.core.BlockHeaderBuilder;
import org.hyperledger.besu.ethereum.core.Difficulty;
import org.hyperledger.besu.ethereum.mainnet.MainnetBlockHeaderFunctions;

public record BlockHeaderSnapshot(
    String parentHash,
    String ommersHash,
    String coinbase,
    String stateRoot,
    String transactionRoot,
    String receiptsRoot,
    String logsBloom,
    String difficulty,
    long number,
    long gasLimit,
    long gasUsed,
    long timestamp,
    String extraData,
    String mixHashOrPrevRandao,
    long nonce,
    Optional<String> baseFee,
    Optional<String> parentBeaconBlockRoot) {
  public static BlockHeaderSnapshot from(BlockHeader header) {
    return new BlockHeaderSnapshot(
        header.getParentHash().getBytes().toHexString(),
        header.getOmmersHash().getBytes().toHexString(),
        header.getCoinbase().getBytes().toHexString(),
        header.getStateRoot().getBytes().toHexString(),
        header.getTransactionsRoot().getBytes().toHexString(),
        header.getReceiptsRoot().getBytes().toHexString(),
        header.getLogsBloom().getBytes().toHexString(),
        header.getDifficulty().toHexString(),
        header.getNumber(),
        header.getGasLimit(),
        header.getGasUsed(),
        header.getTimestamp(),
        header.getExtraData().toHexString(),
        header.getMixHashOrPrevRandao().toHexString(),
        header.getNonce(),
        header.getBaseFee().map(Quantity::toHexString),
        header.getParentBeaconBlockRoot().map(Bytes::toHexString));
  }

  public BlockHeader toBlockHeader() {
    final BlockHeaderBuilder builder =
        BlockHeaderBuilder.create()
            .parentHash(Hash.fromHexString(this.parentHash))
            .ommersHash(Hash.fromHexString(this.ommersHash))
            .coinbase(Address.fromHexString(this.coinbase))
            .stateRoot(Hash.fromHexString(this.stateRoot))
            .transactionsRoot(Hash.fromHexString(this.transactionRoot))
            .receiptsRoot(Hash.fromHexString(this.receiptsRoot))
            .logsBloom(LogsBloomFilter.fromHexString(this.logsBloom))
            .difficulty(Difficulty.fromHexString(this.difficulty))
            .number(this.number)
            .gasLimit(this.gasLimit)
            .gasUsed(this.gasUsed)
            .timestamp(this.timestamp)
            .extraData(Bytes.fromHexString(this.extraData))
            .mixHash(Hash.fromHexString(this.mixHashOrPrevRandao))
            .prevRandao(Bytes32.fromHexString(this.mixHashOrPrevRandao))
            .nonce(this.nonce)
            .blockHeaderFunctions(new MainnetBlockHeaderFunctions());

    this.baseFee.ifPresent(baseFee -> builder.baseFee(Wei.fromHexString(baseFee)));
    // Following null check is required for older replays only.  Eventually, it can be removed (i.e.
    // once all LONDON replays are dropped).  Also, this is necessary despite the fact that this
    // field
    // is already an Optional.
    if (this.parentBeaconBlockRoot != null) {
      this.parentBeaconBlockRoot.ifPresent(
          root -> builder.parentBeaconBlockRoot(Bytes32.fromHexString(root)));
    } else {
      // NOTE: following is to enable older replays to work with later forks.
      builder.parentBeaconBlockRoot(
          Bytes32.fromHexString(
              "0x0000000000000000000000000000000000000000000000000000000000000000"));
    }
    //
    return builder.buildBlockHeader();
  }
}
