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
package net.consensys.linea.zktracer;

import static net.consensys.linea.zktracer.ChainConfig.FORK_LINEA_CHAIN;
import static net.consensys.linea.zktracer.Fork.getTraceFromFork;
import static net.consensys.linea.zktracer.types.PublicInputs.emptyHistoricalBlockhashes;
import static net.consensys.linea.zktracer.types.PublicInputs.emptyPublicInputs;

import java.io.BufferedOutputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.math.BigInteger;
import java.nio.file.Path;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.zip.GZIPOutputStream;
import lombok.Getter;
import lombok.Setter;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.zktracer.container.module.Module;
import net.consensys.linea.zktracer.exceptions.TracingExceptions;
import net.consensys.linea.zktracer.lt.LtFile;
import net.consensys.linea.zktracer.module.DebugMode;
import net.consensys.linea.zktracer.module.hub.*;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import net.consensys.linea.zktracer.types.FiniteList;
import net.consensys.linea.zktracer.types.PublicInputs;
import org.apache.commons.io.FilenameUtils;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.log.Log;
import org.hyperledger.besu.evm.operation.Operation;
import org.hyperledger.besu.evm.worldstate.WorldView;
import org.hyperledger.besu.plugin.data.BlockBody;
import org.hyperledger.besu.plugin.data.BlockHeader;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;

@Slf4j
public class ZkTracer implements LineCountingTracer {

  /** Construct trace object. */
  private final Trace trace;

  @Getter private final Hub hub;
  private final Optional<DebugMode> debugMode;

  /** Accumulate all the exceptions that happened at tracing time. */
  @Getter private final List<Exception> tracingExceptions = new FiniteList<>(50);

  // Fields for metadata
  @Getter private final ChainConfig chain;

  /** Determines which version the LT trace file format to use by default. */
  @Setter private int ltFileMajorVersion = 2;

  /**
   * Construct a ZkTracer for a given bridge configuration and chainId. This is used, for example,
   * by the coordinator for tracing in production, such as on mainnet and/or sepolia.
   *
   * @param bridgeConfiguration Configuration for the L1L2 bridge.
   * @param chainId Identifies the chain being traced.
   */
  public ZkTracer(
      final Fork fork,
      final LineaL1L2BridgeSharedConfiguration bridgeConfiguration,
      BigInteger chainId,
      PublicInputs publicInputs) {
    this(FORK_LINEA_CHAIN(fork, bridgeConfiguration, chainId), publicInputs);
  }

  /**
   * Construct a ZkTracer for a given bridge configuration and chainId, without a BlockchainService.
   * This is used, for example, by the sequencer for line counting in production, such as on mainnet
   * and/or sepolia, or in tests.
   *
   * @param bridgeConfiguration Configuration for the L1L2 bridge.
   * @param chainId Identifies the chain being traced.
   */
  public ZkTracer(
      final Fork fork,
      final LineaL1L2BridgeSharedConfiguration bridgeConfiguration,
      BigInteger chainId) {
    this(FORK_LINEA_CHAIN(fork, bridgeConfiguration, chainId));
  }

  public ZkTracer(ChainConfig chain, Map<Long, Bytes> blobBaseFees) {
    this(chain, emptyHistoricalBlockhashes(blobBaseFees));
  }

  public ZkTracer(ChainConfig chain) {
    this(chain, emptyPublicInputs());
  }

  /**
   * Construct a ZkTracer with a given chain configuration, which could either for a production
   * environment or a test environment.
   *
   * @param chain
   */
  public ZkTracer(ChainConfig chain, PublicInputs publicInputs) {
    if (publicInputs.missingSomeInfos()) {
      log.info(
          "[ZkTracer] Missing part of the public inputs, assuming line counting only, testing, or tracing very specific conflation. Tracing might fail. \nhistorical blockhashes size = {}\nblob base fees size = {}",
          publicInputs.historicalBlockhashes() == null
              ? 0
              : publicInputs.historicalBlockhashes().size(),
          publicInputs.blobBaseFees() == null ? 0 : publicInputs.blobBaseFees().size());
    }
    this.chain = chain;
    hub =
        switch (chain.fork) {
          case OSAKA -> new Hub(chain, publicInputs);
          case LONDON, PARIS, SHANGHAI, CANCUN, PRAGUE ->
              throw new IllegalArgumentException(
                  "Fork no more supported by the tracer: " + chain.fork);
          default -> throw new IllegalArgumentException("Unknown fork: " + chain.fork);
        };
    trace = getTraceFromFork(chain.fork);
    final DebugMode.PinLevel debugLevel = new DebugMode.PinLevel();
    this.debugMode =
        debugLevel.none() ? Optional.empty() : Optional.of(new DebugMode(debugLevel, this.hub));

    log.info("[ZkTracer] Created ZkTracer for fork {}", chain.fork);
  }

  public void writeToFile(final Path filename, long startBlock, long endBlock) {
    maybeThrowTracingExceptions();
    final List<Module> modulesToTrace = hub.getModulesToTrace();
    // Configure metadata
    final Map<String, Object> metadata = buildMetaData(startBlock, endBlock);
    // Construct (in memory) trace file
    LtFile.Header lth = new LtFile.Header(ltFileMajorVersion, 0, metadata);
    LtFile ltf = LtFile.of(lth, trace, modulesToTrace);
    //
    try (FileOutputStream fout = new FileOutputStream(filename.toString())) {
      // Compress file (if requested)
      if (FilenameUtils.getExtension(filename.toString()).equals("gz")) {
        // GZIPOutputStream has an internal buffer, so no need for separate buffered output stream.
        try (GZIPOutputStream gzOut = new GZIPOutputStream(fout, 65536)) {
          ltf.write(gzOut);
          gzOut.flush();
        }
      } else {
        try (BufferedOutputStream bout = new BufferedOutputStream(fout, 65536)) {
          ltf.write(bout);
          bout.flush();
        }
      }
      // Flush to disk
      fout.flush();
    } catch (IOException e) {
      log.error("Error while writing file {}", filename);
      throw new RuntimeException(e);
    }
  }

  /**
   * Construct the metadata to be embedded in the generated trace file.
   *
   * @param startBlock
   * @param endBlock
   * @return
   */
  private Map<String, Object> buildMetaData(long startBlock, long endBlock) {
    HashMap<String, Object> metadata = new HashMap<>(trace.getMetaData());
    metadata.put("releaseVersion", ZkTracer.class.getPackage().getSpecificationVersion());
    metadata.put("chainId", this.chain.id.toString());
    metadata.put("fork", this.chain.fork.toString());
    metadata.put("l2L1LogSmcAddress", this.chain.bridgeConfiguration.contract().toString());
    metadata.put("l2L1LogTopic", this.chain.bridgeConfiguration.topic().toString());
    // include block range
    final Map<String, String> range = new HashMap<>();
    range.put("start", Long.toString(startBlock));
    range.put("end", Long.toString(endBlock));
    metadata.put("conflation", range);
    // include line counts
    final Map<String, String> lineCounts = new HashMap<>();
    for (Module m : hub.getTracelessModules()) {
      lineCounts.put(m.moduleKey().toString(), Integer.toString(m.lineCount()));
    }
    metadata.put("lineCounts", lineCounts);
    //
    return metadata;
  }

  @Override
  public void traceStartConflation(final long numBlocksInConflation) {
    try {
      hub.traceStartConflation(numBlocksInConflation);
      this.debugMode.ifPresent(x -> x.traceStartConflation(numBlocksInConflation));
    } catch (final Exception e) {
      collectException(e);
    }
  }

  @Override
  public void traceEndConflation(final WorldView state) {
    try {
      this.hub.traceEndConflation(state);
      this.debugMode.ifPresent(DebugMode::traceEndConflation);
    } catch (final Exception e) {
      collectException(e);
    }

    if (!this.tracingExceptions.isEmpty()) {
      throw new TracingExceptions(this.tracingExceptions);
    }
  }

  @Override
  public void traceStartBlock(
      WorldView world,
      final ProcessableBlockHeader processableBlockHeader,
      final Address miningBeneficiary) {
    try {
      this.hub.traceStartBlock(world, processableBlockHeader, miningBeneficiary);
      this.debugMode.ifPresent(DebugMode::traceEndConflation);
    } catch (final Exception e) {
      collectException(e);
    }
  }

  @Override
  public void traceStartBlock(
      WorldView world,
      final BlockHeader blockHeader,
      final BlockBody blockBody,
      final Address miningBeneficiary) {
    try {
      this.hub.traceStartBlock(world, blockHeader, miningBeneficiary);
      this.debugMode.ifPresent(x -> x.traceStartBlock(blockHeader, blockBody, miningBeneficiary));
    } catch (final Exception e) {
      collectException(e);
    }
  }

  @Override
  public void traceEndBlock(final BlockHeader blockHeader, final BlockBody blockBody) {
    try {
      this.hub.traceEndBlock(blockHeader, blockBody);
      this.debugMode.ifPresent(DebugMode::traceEndBlock);
    } catch (final Exception e) {
      collectException(e);
    }
  }

  public void tracePrepareTransaction(WorldView worldView, Transaction transaction) {
    try {
      this.debugMode.ifPresent(x -> x.tracePrepareTx(worldView, transaction));
      this.hub.traceStartTransaction(worldView, transaction);
    } catch (final Exception e) {
      collectException(e);
    }
  }

  public void traceEndTransaction(
      WorldView worldView,
      Transaction tx,
      boolean status,
      Bytes output,
      List<Log> logs,
      long gasUsed,
      Set<Address> selfDestructs,
      long timeNs) {
    try {
      this.debugMode.ifPresent(x -> x.traceEndTx(worldView, tx, status, output, logs, gasUsed));
      this.hub.traceEndTransaction(worldView, tx, status, logs, selfDestructs);
    } catch (final Exception e) {
      collectException(e);
    }
  }

  /**
   * Linea's zkEVM does not trace the STOP instruction of either (a) CALL's to accounts with empty
   * byte code (b) CREATE's with empty initialization code.
   *
   * <p>Note however that the relevant {@link CallFrame}'s are (and SHOULD BE) created regardless.
   *
   * @param frame
   */
  @Override
  public void tracePreExecution(final MessageFrame frame) {
    this.hub.currentFrame().frame(frame);
    if (frame.getCode().getSize() > 0) {
      try {
        this.hub.tracePreExecution(frame);
        this.debugMode.ifPresent(x -> x.tracePreOpcode(frame));
      } catch (final Exception e) {
        collectException(e);
      }
    }
  }

  /**
   * Compare with description of {@link #tracePreExecution(MessageFrame)}.
   *
   * @param frame
   * @param operationResult
   */
  @Override
  public void tracePostExecution(MessageFrame frame, Operation.OperationResult operationResult) {
    if (frame.getCode().getSize() > 0) {
      try {
        this.hub.tracePostExecution(frame, operationResult);
        this.debugMode.ifPresent(x -> x.tracePostOpcode(frame, operationResult));
      } catch (final Exception e) {
        collectException(e);
      }
    }
  }

  @Override
  public void traceContextEnter(MessageFrame frame) {
    // We only want to trigger on creation of new contexts, not on re-entry in
    // existing contexts
    if (frame.getState() == MessageFrame.State.NOT_STARTED) {
      try {
        this.hub.traceContextEnter(frame);
        this.debugMode.ifPresent(x -> x.traceContextEnter(frame));
      } catch (final Exception e) {
        collectException(e);
      }
    }
  }

  @Override
  public void traceContextReEnter(MessageFrame frame) {
    try {
      this.hub.traceContextReEnter(frame);
      this.debugMode.ifPresent(x -> x.traceContextReEnter(frame));
    } catch (final Exception e) {
      collectException(e);
    }
  }

  @Override
  public void traceContextExit(MessageFrame frame) {
    try {
      this.hub.traceContextExit(frame);
      this.debugMode.ifPresent(x -> x.traceContextExit(frame));
    } catch (final Exception e) {
      collectException(e);
    }
  }

  private void collectException(final Exception e) {
    this.tracingExceptions.add(e);
    log.trace("Collected exception during transaction processing", e);
  }

  private void maybeThrowTracingExceptions() {
    if (!this.tracingExceptions.isEmpty()) {
      throw new TracingExceptions(this.tracingExceptions);
    }
  }

  /** When called, erase all tracing related to the bundle of all transactions since the last. */
  @Override
  public void popTransactionBundle() {
    log.info(
        "[ZkTracer] Transaction bundle has been popped, assuming block building. Tracing might fail.");
    hub.popTransactionBundle();
  }

  @Override
  public void commitTransactionBundle() {
    hub.commitTransactionBundle();
  }

  /**
   * Returns the total line count (i.e. including spillage) for both tracing and non-tracing
   * modules. This method is called directly by the sequencer to determine whether a given
   * transaction should go ahead. This method is also used to feed the line counting RPC end points.
   *
   * @return
   */
  @Override
  public Map<String, Integer> getModulesLineCount() {
    maybeThrowTracingExceptions();
    final HashMap<String, Integer> modulesLineCount = new HashMap<>();

    for (Module m : hub.getModulesToCount()) {
      modulesLineCount.put(m.moduleKey().toString(), m.lineCount() + m.spillage(this.trace));
    }
    //
    return modulesLineCount;
  }

  @Override
  public List<Module> getModulesToCount() {
    return hub.getModulesToCount();
  }

  public Set<Address> getAddressesSeenByHubForRelativeBlock(final int relativeBlockNumber) {
    return hub.blockStack().getBlockByRelativeBlockNumber(relativeBlockNumber).addressesSeenByHub();
  }

  public Map<Address, Set<Bytes32>> getStoragesSeenByHubForRelativeBlock(
      final int relativeBlockNumber) {
    return hub.blockStack().getBlockByRelativeBlockNumber(relativeBlockNumber).storagesSeenByHub();
  }
}
