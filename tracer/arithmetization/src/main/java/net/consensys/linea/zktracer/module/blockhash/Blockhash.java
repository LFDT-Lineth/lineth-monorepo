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

package net.consensys.linea.zktracer.module.blockhash;

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.module.ModuleName.BLOCK_HASH;
import static net.consensys.linea.zktracer.module.blockhash.BlockhashOperation.NB_ROWS_BLOCKHASH;
import static net.consensys.linea.zktracer.types.Conversions.longToBytes32;

import java.util.HashMap;
import java.util.List;
import java.util.Map;
import lombok.Getter;
import lombok.experimental.Accessors;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.OperationSetModule;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationStackedSet;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.defer.PostOpcodeDefer;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.operation.Operation;
import org.hyperledger.besu.evm.worldstate.WorldView;
import org.hyperledger.besu.plugin.data.BlockBody;
import org.hyperledger.besu.plugin.data.BlockHeader;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;

@Slf4j
@Getter
@Accessors(fluent = true)
public class Blockhash implements OperationSetModule<BlockhashOperation>, PostOpcodeDefer {
  private final Hub hub;
  private final Wcp wcp;
  private final ModuleOperationStackedSet<BlockhashOperation> operations =
      new ModuleOperationStackedSet<>();
  List<BlockhashOperation> sortedOperations;

  /* Stores the result of BLOCKHASH if the result of the opcode is not 0 */
  private final Map<Long, Hash> blockHashMap;
  private final Map<Long, Boolean> successfulBlockhashAttempt = new HashMap<>();
  private Hash lastBlockHash;

  private long firstBlockOfConflation = -1;
  private long absBlock;
  private Bytes32 blockhashArg;

  public Blockhash(Hub hub, Wcp wcp, Map<Long, Hash> historicalBlockHashes) {
    blockHashMap = historicalBlockHashes;
    this.hub = hub;
    this.wcp = wcp;
  }

  @Override
  public ModuleName moduleKey() {
    return BLOCK_HASH;
  }

  @Override
  public void traceStartBlock(
      WorldView world,
      final ProcessableBlockHeader processableBlockHeader,
      final Address miningBeneficiary) {
    absBlock = processableBlockHeader.getNumber();
    if (firstBlockOfConflation == -1) {
      firstBlockOfConflation = absBlock;
    }
  }

  @Override
  public void traceEndBlock(final BlockHeader blockHeader, final BlockBody blockBody) {
    lastBlockHash = blockHeader.getBlockHash();
  }

  public void callBlockhash(MessageFrame frame) {
    blockhashArg = Bytes32.leftPad(frame.getStackItem(0));
    hub.defers().scheduleForPostExecution(this);
  }

  public void callBlockhashForParent(ProcessableBlockHeader blockHeader) {
    final long blockNumber = blockHeader.getNumber();
    checkArgument(blockNumber > 0, "BLOCKHASH can't be called for genesis block");
    final Bytes32 parentBlockNumber = longToBytes32(blockNumber - 1);
    final Hash parentBlockHash = blockHeader.getParentHash();
    final BlockhashOperation op =
        new BlockhashOperation(
            fromAbsoluteBlockToRelativeBlock(blockNumber),
            blockNumber,
            parentBlockNumber,
            parentBlockHash,
            wcp);
    addAndCheck(op);
  }

  @Override
  public void resolvePostExecution(
      Hub hub, MessageFrame frame, Operation.OperationResult operationResult) {

    final Hash blockhashRes = Hash.wrap(Bytes32.leftPad(frame.getStackItem(0)));
    final BlockhashOperation op =
        new BlockhashOperation(
            fromAbsoluteBlockToRelativeBlock(absBlock), absBlock, blockhashArg, blockhashRes, wcp);
    addAndCheck(op);
  }

  private void addAndCheck(BlockhashOperation e) {
    operations.add(e);
    checkBlockHashConsistencies(e);
  }

  private void checkBlockHashConsistencies(BlockhashOperation op) {
    // We have 4 LLARGE and one OLI call to WCP, made at the end of the conflation, so we need to
    // add line count to WCP
    wcp.additionalRows.add(4 * LLARGE + 1);

    // check that the result is coherent with what we know
    if (!op.blockhashRes().getBytes().equals(Bytes32.ZERO)) {
      checkArgument(
          op.blockhashArg().trimLeadingZeros().size() <= 8, "Block number must fit in a long");
      final long blockNumber = op.blockhashArg().toLong();
      successfulBlockhashAttempt.putIfAbsent(blockNumber, true);
      if (blockHashMap.containsKey(blockNumber)) {
        checkArgument(
            op.blockhashRes().getBytes().equals(blockHashMap.get(blockNumber).getBytes()));
      } else {
        blockHashMap.put(blockNumber, op.blockhashRes());
      }
    }
  }

  /**
   * Operations are sorted wrt blockhashArg and the wcp module is called accordingly. We must call
   * the WCP module before calling {@link #commit(Trace)} as the headers sizes must be computed with
   * the final list of operations ready.
   */
  @Override
  public void traceEndConflation(WorldView state) {
    // Add all historical block hashes if not already called by the EVM:
    for (long blockNumber : blockHashMap.keySet()) {
      if (!successfulBlockhashAttempt.getOrDefault(blockNumber, false)) {
        final long absoluteBlock = Math.max(firstBlockOfConflation, blockNumber + 1);

        final BlockhashOperation op =
            new BlockhashOperation(
                fromAbsoluteBlockToRelativeBlock(absoluteBlock),
                absoluteBlock,
                longToBytes32(blockNumber),
                blockHashMap.get(blockNumber), // We add successful calls only
                wcp);
        operations.add(op);
      }
    }

    // Add the blockhash of the last block. Its treatment is different from other historical
    // blockhashes, as its value can't be known during execution
    final BlockhashOperation lastBlockHash =
        new BlockhashOperation(
            fromAbsoluteBlockToRelativeBlock(absBlock),
            absBlock,
            longToBytes32(absBlock),
            Hash.ZERO,
            wcp);
    operations.add(lastBlockHash);
    checkArgument(
        blockHashMap.get(absBlock) == null,
        "The blockhash of the last block can't be known during execution");
    blockHashMap.put(absBlock, this.lastBlockHash);

    // Sort and trace operations
    OperationSetModule.super.traceEndConflation(state);
    sortedOperations = sortOperations(new BlockhashComparator());
    Bytes32 prevBlockhashArg = Bytes32.ZERO;
    for (BlockhashOperation op : sortedOperations) {
      op.handlePreprocessing(prevBlockhashArg);
      prevBlockhashArg = op.blockhashArg();
    }
  }

  @Override
  public List<Trace.ColumnHeader> columnHeaders(Trace trace) {
    return trace.blockhash().headers(this.lineCount());
  }

  @Override
  public int spillage(Trace trace) {
    return trace.blockhash().spillage();
  }

  @Override
  public void commit(Trace trace) {
    for (BlockhashOperation op : sortedOperations) {
      final Hash blockhashVal =
          op.blockhashArg().trimLeadingZeros().size() <= 8
              ? blockHashMap.getOrDefault(op.blockhashArg().trimLeadingZeros().toLong(), Hash.ZERO)
              : Hash.ZERO;
      op.traceMacro(trace.blockhash(), Bytes32.wrap(blockhashVal.getBytes()));
      op.tracePreprocessing(trace.blockhash());
    }
  }

  @Override
  public int lineCount() {
    final int additionalOp =
        operations.conflationFinished()
            ? 0
            : Math.max(0, blockHashMap.size() + 1 - successfulBlockhashAttempt().size());
    return operations().lineCount() + additionalOp * NB_ROWS_BLOCKHASH;
  }

  public short fromAbsoluteBlockToRelativeBlock(long absoluteBlock) {
    return (short) ((absoluteBlock - firstBlockOfConflation) + 1);
  }
}
