/*
 * Copyright ConsenSys Inc.
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
package net.consensys.linea.zktracer.module.blockdata.module;

import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.module.ModuleName.BLOCK_DATA;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;

import java.util.*;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import net.consensys.linea.zktracer.ChainConfig;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.Module;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.module.blockdata.moduleInstruction.BaseFeeInstruction;
import net.consensys.linea.zktracer.module.blockdata.moduleInstruction.BlobBaseFeeInstruction;
import net.consensys.linea.zktracer.module.blockdata.moduleInstruction.BlockDataInstruction;
import net.consensys.linea.zktracer.module.blockdata.moduleInstruction.ChainIdInstruction;
import net.consensys.linea.zktracer.module.blockdata.moduleInstruction.CoinbaseInstruction;
import net.consensys.linea.zktracer.module.blockdata.moduleInstruction.GasLimitInstruction;
import net.consensys.linea.zktracer.module.blockdata.moduleInstruction.NumberInstruction;
import net.consensys.linea.zktracer.module.blockdata.moduleInstruction.PrevRandaoInstruction;
import net.consensys.linea.zktracer.module.blockdata.moduleInstruction.TimestampInstruction;
import net.consensys.linea.zktracer.module.euc.Euc;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.worldstate.WorldView;
import org.hyperledger.besu.plugin.data.BlockBody;
import org.hyperledger.besu.plugin.data.BlockHeader;

@RequiredArgsConstructor
public abstract class BlockData implements Module {
  private final Hub hub;
  private final Wcp wcp;
  private final Euc euc;
  @Getter private final ChainConfig chain;
  protected final Map<Long, Bytes> blobBaseFees;
  @Getter public final Map<Long, List<BlockDataInstruction>> InstructionsPerBlock = new TreeMap<>();
  @Getter public long firstBlockNumber;
  public long blockTimestamp;
  public long blockNumber;

  private boolean conflationFinished = false;

  private final OpCode[] opCodes = setOpCodes();

  @Override
  public ModuleName moduleKey() {
    return BLOCK_DATA;
  }

  @Override
  public void traceStartConflation(final long blockCount) {
    wcp.additionalRows.add(
        LLARGE // for COINBASE
            + 6
            + 6 // for TIMESTAMP
            + 1
            + 6 // for NUMBER
            + 1 // for DIFFICULTY or PREVRANDAO
            + (bigIntegerToBytes(chain.gasLimitMaximum).size() * 4) // for GASLIMIT
            + LLARGE // for CHAINID
            + LLARGE // for BASEFEE
        // TODO: we should add +1 here for BLOBBASEFEE (post Cancun), but we don't as limitless
        // prover will be deployed before Cancun (and all this line counting will die)
        );

    euc.additionalRows.add(8);
  }

  @Override
  public void traceEndConflation(final WorldView state) {
    conflationFinished = true;
  }

  @Override
  public void traceEndBlock(final BlockHeader blockHeader, final BlockBody blockBody) {
    blockNumber = blockHeader.getNumber();
    blockTimestamp = blockHeader.getTimestamp();
    if (InstructionsPerBlock.isEmpty()) {
      firstBlockNumber = blockNumber;
    }
    final BlockHeader previousBlockHeader =
        InstructionsPerBlock.isEmpty()
            ? null
            : InstructionsPerBlock.get(blockNumber - 1).getFirst().blockHeader;
    List<BlockDataInstruction> blockDataInstructionList = new ArrayList<>();
    for (OpCode opCode : opCodes) {
      BlockDataInstruction blockDataInstruction =
          getInstruction(opCode, blockHeader, previousBlockHeader);
      blockDataInstruction.handle();
      blockDataInstructionList.addLast(blockDataInstruction);
    }
    InstructionsPerBlock.put(blockNumber, blockDataInstructionList);
  }

  protected abstract OpCode[] setOpCodes();

  @Override
  public void commitTransactionBundle() {}

  @Override
  public void popTransactionBundle() {}

  @Override
  public int lineCount() {
    final int numberOfBlock = InstructionsPerBlock.size() + (conflationFinished ? 0 : 1);
    return numberOfBlock * numberOfLinesPerBlock();
  }

  protected abstract int numberOfLinesPerBlock();

  @Override
  public int spillage(Trace trace) {
    return trace.blockdata().spillage();
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.blockdata().headers(this.lineCount());
  }

  protected abstract boolean shouldTraceTimestampAndNumber();

  protected abstract boolean shouldTraceRelTxNumMax();

  @Override
  public void commit(Trace trace) {
    for (Map.Entry<Long, List<BlockDataInstruction>> entry : InstructionsPerBlock.entrySet()) {
      List<BlockDataInstruction> value = entry.getValue();
      for (BlockDataInstruction blockDataInstruction : value) {
        Trace.Blockdata traceBlockdata = trace.blockdata();
        blockDataInstruction.trace(
            traceBlockdata, shouldTraceTimestampAndNumber(), shouldTraceRelTxNumMax());
      }
    }
  }

  public BlockDataInstruction getInstruction(
      OpCode opCode, BlockHeader blockHeader, BlockHeader prevBlockHeader) {
    return switch (opCode) {
      case COINBASE ->
          new CoinbaseInstruction(
              chain, hub, wcp, euc, blockHeader, prevBlockHeader, firstBlockNumber);
      case TIMESTAMP ->
          new TimestampInstruction(
              chain, hub, wcp, euc, blockHeader, prevBlockHeader, firstBlockNumber);
      case NUMBER ->
          new NumberInstruction(
              chain, hub, wcp, euc, blockHeader, prevBlockHeader, firstBlockNumber);
      case PREVRANDAO ->
          new PrevRandaoInstruction(
              chain, hub, wcp, euc, blockHeader, prevBlockHeader, firstBlockNumber);
      case GASLIMIT ->
          new GasLimitInstruction(
              chain, hub, wcp, euc, blockHeader, prevBlockHeader, firstBlockNumber);
      case CHAINID ->
          new ChainIdInstruction(
              chain, hub, wcp, euc, blockHeader, prevBlockHeader, firstBlockNumber);
      case BASEFEE ->
          new BaseFeeInstruction(
              chain, hub, wcp, euc, blockHeader, prevBlockHeader, firstBlockNumber);
      case BLOBBASEFEE ->
          new BlobBaseFeeInstruction(
              chain, hub, wcp, euc, blockHeader, prevBlockHeader, firstBlockNumber, blobBaseFees);
      default -> throw new IllegalArgumentException("[BlockData] Unsupported opcode " + opCode);
    };
  }
}
