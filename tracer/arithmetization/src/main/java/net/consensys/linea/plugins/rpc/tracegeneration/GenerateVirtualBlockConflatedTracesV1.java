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

package net.consensys.linea.plugins.rpc.tracegeneration;

import static net.consensys.linea.zktracer.Fork.getForkFromBesuBlockchainService;
import static net.consensys.linea.zktracer.types.PublicInputs.generatePublicInputs;

import com.google.common.base.Stopwatch;
import java.math.BigInteger;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.Objects;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.plugins.BesuServiceProvider;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.plugins.rpc.RequestLimiter;
import net.consensys.linea.plugins.rpc.Validator;
import net.consensys.linea.tracewriter.TraceWriter;
import net.consensys.linea.zktracer.Fork;
import net.consensys.linea.zktracer.ZkTracer;
import net.consensys.linea.zktracer.json.JsonConverter;
import net.consensys.linea.zktracer.types.PublicInputs;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.StateOverrideMap;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.api.jsonrpc.internal.response.RpcErrorType;
import org.hyperledger.besu.ethereum.api.util.DomainObjectDecodeUtils;
import org.hyperledger.besu.evm.account.Account;
import org.hyperledger.besu.evm.worldstate.WorldView;
import org.hyperledger.besu.plugin.ServiceManager;
import org.hyperledger.besu.plugin.data.BlockContext;
import org.hyperledger.besu.plugin.data.BlockOverrides;
import org.hyperledger.besu.plugin.data.PluginBlockSimulationResult;
import org.hyperledger.besu.plugin.services.BlockSimulationService;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.hyperledger.besu.plugin.services.exception.PluginRpcEndpointException;
import org.hyperledger.besu.plugin.services.rpc.PluginRpcRequest;

/**
 * RPC endpoint for generating conflated traces for virtual blocks. This is used for invalidity
 * proof generation for BadPrecompile and TooManyLogs forced transaction rejection scenarios.
 *
 * <p>The endpoint simulates execution of transactions on top of a past chain state (blockNumber -
 * 1) and generates execution traces that can be used for ZK proof generation
 */
@Slf4j
public class GenerateVirtualBlockConflatedTracesV1 {
  private static final JsonConverter CONVERTER = JsonConverter.builder().build();

  private final boolean traceFileCaching;
  private final int traceFileVersion;
  private final RequestLimiter requestLimiter;
  private final TraceWriter traceWriter;
  private final LineaL1L2BridgeSharedConfiguration l1L2BridgeSharedConfiguration;
  private final BlockchainService blockchainService;
  private final ServiceManager besuContext;

  public GenerateVirtualBlockConflatedTracesV1(
      final RequestLimiter requestLimiter,
      final TracesEndpointConfiguration endpointConfiguration,
      final LineaL1L2BridgeSharedConfiguration lineaL1L2BridgeSharedConfiguration,
      final ServiceManager besuContext) {
    this.requestLimiter = requestLimiter;
    this.traceWriter =
        new TraceWriter(
            Paths.get(endpointConfiguration.tracesOutputPath()),
            endpointConfiguration.traceCompression());
    this.l1L2BridgeSharedConfiguration = lineaL1L2BridgeSharedConfiguration;
    this.traceFileVersion = endpointConfiguration.traceFileVersion();
    this.traceFileCaching = endpointConfiguration.caching();
    this.blockchainService =
        BesuServiceProvider.getBesuService(besuContext, BlockchainService.class);
    this.besuContext = besuContext;
  }

  public String getNamespace() {
    return "linea";
  }

  public String getName() {
    return "generateVirtualBlockConflatedTracesToFileV1";
  }

  /**
   * Handles virtual block execution traces generation.
   *
   * @param request holds parameters of the RPC request.
   * @return an execution file trace.
   */
  public TraceFile execute(final PluginRpcRequest request) {
    return requestLimiter.execute(request, this::generateVirtualBlockTraceFile);
  }

  private TraceFile generateVirtualBlockTraceFile(PluginRpcRequest request) {
    final Stopwatch sw = Stopwatch.createStarted();

    final Object[] rawParams = request.getParams();
    Validator.validatePluginRpcRequestParams(rawParams);

    final VirtualBlockTraceRequestParams params =
        CONVERTER.fromJson(CONVERTER.toJson(rawParams[0]), VirtualBlockTraceRequestParams.class);

    params.validate();

    final long blockNumber = params.blockNumber();
    final long parentBlockNumber = blockNumber - 1;

    // It's "unknown" for the test env, because there's no jar
    final String tracesEngineVersion =
        Objects.requireNonNullElse(TraceRequestParams.getTracerRuntime(), "unknown");

    // Hash of the caller-supplied transactions — makes the cache key unique per transaction set
    final String txsHash = computeTxsHash(params.txsRlpEncoded());

    // Check for cached trace file
    Path path = traceWriter.virtualBlockTraceFilePath(blockNumber, tracesEngineVersion, txsHash);
    if (cachedTraceFileAvailable(path)) {
      log.info("virtual block trace cache hit: blockNumber={} path={}", blockNumber, path);
      return new TraceFile(tracesEngineVersion, path.toString());
    }

    // Validate parent block exists
    final BlockContext parentBlock =
        blockchainService
            .getBlockByNumber(parentBlockNumber)
            .orElseThrow(
                () ->
                    new PluginRpcEndpointException(
                        RpcErrorType.BLOCK_NOT_FOUND,
                        "parent block %d not found (required for virtual block %d)"
                            .formatted(parentBlockNumber, blockNumber)));

    log.info(
        "generating virtual block traces: blockNumber={} parentBlockNumber={}",
        blockNumber,
        parentBlockNumber);

    // Decode RLP transactions
    final List<Transaction> transactions = decodeTransactions(params.txsRlpEncoded());
    log.debug("decoded transactions: count={} blockNumber={}", transactions.size(), blockNumber);

    // Get fork for the virtual block
    final Fork fork = getForkFromBesuBlockchainService(blockchainService, blockNumber, blockNumber);
    final BigInteger chainId =
        blockchainService
            .getChainId()
            .orElseThrow(() -> new IllegalStateException("ChainId must be provided"));

    // Public inputs: use real historical block hashes from the canonical chain, matching what
    // GenerateConflatedTracesV2 does.
    final PublicInputs publicInputs =
        generatePublicInputs(blockchainService, blockNumber, blockNumber);

    // Build block overrides for the virtual block.
    // When the canonical block already exists in the chain, mirror its exact header so that the
    // block context (timestamp, coinbase, gasLimit, baseFee, prevRandao, beaconRoot) is identical.
    // This matters for the EIP-4788 / EIP-2935 system-transaction traces in Hub.traceStartBlock.
    final BlockOverrides blockOverrides =
        buildBlockOverrides(blockNumber, parentBlock, blockchainService);

    // Create ZkTracer
    final ZkTracer tracer =
        new ZkTracer(fork, l1L2BridgeSharedConfiguration, chainId, publicInputs);
    tracer.setLtFileMajorVersion(traceFileVersion);

    final BlockSimulationService blockSimulationService =
        BesuServiceProvider.getBesuService(besuContext, BlockSimulationService.class);

    // Start conflation for single block.
    // Note: traceStartBlock / traceEndBlock are NOT called manually here because
    // BlockSimulationService.simulate() now invokes them internally on BlockAwareOperationTracer
    // instances (added in Besu 3e74fee). Calling them twice would corrupt BlockData state.
    tracer.traceStartConflation(1);

    PluginBlockSimulationResult simulationResult;
    try {
      // Simulate the virtual block with our tracer.
      // Note: BlockSimulationService converts Transaction → CallParameter internally, which
      // strips the ECDSA signature (R, S, V) and substitutes a fake one.  This means the
      // rlptxn module will trace the fake signature rather than the real one.  Whether this
      // is acceptable for ZK proof validity is determined by the circuit — corset validation
      // of this trace will confirm it.
      simulationResult =
          blockSimulationService.simulate(
              parentBlockNumber, transactions, blockOverrides, new StateOverrideMap(), tracer);

      final var simBlockHeader = simulationResult.getBlockHeader();
      final var simBlockBody = simulationResult.getBlockBody();
      log.debug(
          "simulation result: blockHash={} receiptsCount={} txSimResults={} trieLogPresent={}",
          simBlockHeader.getBlockHash(),
          simulationResult.getReceipts().size(),
          simulationResult.getTransactionSimulationResults().size(),
          simulationResult.getTrieLog().isPresent());
      log.debug(
          "simulation block header: number={} blockHash={} parentHash={} coinbase={} timestamp={} gasLimit={} gasUsed={} baseFee={} stateRoot={} transactionsRoot={} receiptsRoot={} prevRandao={} parentBeaconBlockRoot={}",
          simBlockHeader.getNumber(),
          simBlockHeader.getBlockHash(),
          simBlockHeader.getParentHash(),
          simBlockHeader.getCoinbase(),
          simBlockHeader.getTimestamp(),
          simBlockHeader.getGasLimit(),
          simBlockHeader.getGasUsed(),
          simBlockHeader.getBaseFee().orElse(null),
          simBlockHeader.getStateRoot(),
          simBlockHeader.getTransactionsRoot(),
          simBlockHeader.getReceiptsRoot(),
          simBlockHeader.getPrevRandao().orElse(null),
          simBlockHeader.getParentBeaconBlockRoot().orElse(null));
      log.debug("simulation block body: {}", simBlockBody);
      for (int i = 0; i < simulationResult.getTransactionSimulationResults().size(); i++) {
        final var txSimResult = simulationResult.getTransactionSimulationResults().get(i);
        log.debug(
            "tx simulation result: index={} txHash={} successful={} invalid={} gasEstimate={} revertReason={} invalidReason={}",
            i,
            txSimResult.transaction().getHash(),
            txSimResult.isSuccessful(),
            txSimResult.isInvalid(),
            txSimResult.getGasEstimate(),
            txSimResult.getRevertReason().orElse(null),
            txSimResult.getInvalidReason().orElse(null));
      }
      simulationResult
          .getTrieLog()
          .ifPresent(
              trieLog ->
                  log.debug(
                      "trieLog: blockHash={} blockNumber={} accountChanges={} codeChanges={} storageChanges={}",
                      trieLog.getBlockHash(),
                      trieLog.getBlockNumber().orElse(null),
                      trieLog.getAccountChanges().size(),
                      trieLog.getCodeChanges().size(),
                      trieLog.getStorageChanges().size()));

      log.info(
          "virtual block simulation completed: blockNumber={} duration={} blockHash={}",
          blockNumber,
          sw,
          simulationResult.getBlockHeader().getBlockHash());
    } finally {
      // Always finalize the conflation — even on simulation failure — so that the tracer is left
      // in a clean state. EmptyWorldView is safe here because no module reads world state in
      // traceEndConflation.
      tracer.traceEndConflation(EmptyWorldView.INSTANCE);
    }

    sw.reset().start();

    // Write trace file with virtual block naming convention
    path =
        traceWriter.writeVirtualBlockTraceToFile(tracer, blockNumber, tracesEngineVersion, txsHash);

    log.info(
        "virtual block trace serialized: blockNumber={} path={} duration={}",
        blockNumber,
        path,
        sw);

    return new TraceFile(tracesEngineVersion, path.toString());
  }

  /**
   * Builds the {@link BlockOverrides} for the simulated block.
   *
   * <p>If the canonical block for {@code blockNumber} already exists in the chain, its full header
   * is mirrored so that the simulation runs with the same block context (timestamp, coinbase,
   * gasLimit, baseFee, prevRandao, parentBeaconBlockRoot). This ensures the EIP-4788 / EIP-2935
   * system-transaction traces in {@code Hub.traceStartBlock} match the canonical ones.
   *
   * <p>When the block is genuinely non-canonical, sensible defaults derived from the parent block
   * header are used instead.
   */
  private BlockOverrides buildBlockOverrides(
      final long blockNumber,
      final BlockContext parentBlock,
      final BlockchainService blockchainService) {
    final BlockOverrides.Builder builder =
        BlockOverrides.builder()
            .blockNumber(blockNumber)
            .blockHashLookup(n -> getBlockHashByNumber(n, blockchainService));

    final java.util.Optional<BlockContext> maybeCanonical =
        blockchainService.getBlockByNumber(blockNumber);

    if (maybeCanonical.isPresent()) {
      // Mirror the canonical block header so the block context is identical.
      final org.hyperledger.besu.plugin.data.BlockHeader h = maybeCanonical.get().getBlockHeader();
      builder.timestamp(h.getTimestamp()).feeRecipient(h.getCoinbase()).gasLimit(h.getGasLimit());
      h.getBaseFee().ifPresent(bf -> builder.baseFeePerGas(Wei.of(bf.getAsBigInteger())));
      h.getPrevRandao().ifPresent(builder::mixHashOrPrevRandao);
      h.getParentBeaconBlockRoot()
          .ifPresent(r -> builder.parentBeaconBlockRoot((org.apache.tuweni.bytes.Bytes32) r));
    } else {
      // Non-canonical virtual block: derive timestamp from parent, leave other fields as defaults.
      builder.timestamp(parentBlock.getBlockHeader().getTimestamp() + 1);
    }

    return builder.build();
  }

  private boolean cachedTraceFileAvailable(final Path path) {
    if (!Files.exists(path)) {
      return false;
    }
    if (!traceFileCaching) {
      log.info("virtual block trace cache ignored (caching disabled): path={}", path);
      return false;
    }
    return true;
  }

  private static String computeTxsHash(final String[] txsRlpEncoded) {
    return Integer.toHexString(Arrays.hashCode(txsRlpEncoded));
  }

  private List<Transaction> decodeTransactions(String[] txsRlpEncoded) {
    final List<Transaction> transactions = new ArrayList<>(txsRlpEncoded.length);
    for (int i = 0; i < txsRlpEncoded.length; i++) {
      try {
        final Transaction tx = DomainObjectDecodeUtils.decodeRawTransaction(txsRlpEncoded[i]);
        transactions.add(tx);
      } catch (Exception e) {
        throw new IllegalArgumentException(
            "Failed to decode transaction at index " + i + ": " + e.getMessage(), e);
      }
    }
    return transactions;
  }

  private Hash getBlockHashByNumber(long blockNumber, BlockchainService blockchainService) {
    return blockchainService
        .getBlockByNumber(blockNumber)
        .map(block -> block.getBlockHeader().getBlockHash())
        .orElse(Hash.ZERO);
  }

  /**
   * Empty WorldView for traceEndConflation. The actual state is captured during execution via
   * OperationTracer callbacks from BlockSimulationService.
   */
  private enum EmptyWorldView implements WorldView {
    INSTANCE;

    @Override
    public Account get(final Address address) {
      return null;
    }
  }
}
