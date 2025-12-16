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
import static net.consensys.linea.zktracer.types.PublicInputs.*;

import com.google.common.base.Stopwatch;
import java.math.BigInteger;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.Optional;
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
import org.hyperledger.besu.plugin.ServiceManager;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.hyperledger.besu.plugin.services.TraceService;
import org.hyperledger.besu.plugin.services.rpc.PluginRpcRequest;

/**
 * Sets up an RPC endpoint for generating conflated file trace. This class provides an RPC endpoint
 * named 'generateConflatedTracesToFileV0' under the 'rollup' namespace. When this endpoint is
 * called, it triggers the execution of the 'execute' method, which generates conflated file traces
 * based on the provided request parameters and writes them to a file.
 */
@Slf4j
public class GenerateConflatedTracesV2 {
  private static final JsonConverter CONVERTER = JsonConverter.builder().build();

  private final boolean traceFileCaching;
  private final int traceFileVersion;
  private final RequestLimiter requestLimiter;
  private final TraceWriter traceWriter;
  private final ServiceManager besuContext;
  private TraceService traceService;
  private final LineaL1L2BridgeSharedConfiguration l1L2BridgeSharedConfiguration;

  public GenerateConflatedTracesV2(
      final ServiceManager besuContext,
      final RequestLimiter requestLimiter,
      final TracesEndpointConfiguration endpointConfiguration,
      final LineaL1L2BridgeSharedConfiguration lineaL1L2BridgeSharedConfiguration) {
    this.besuContext = besuContext;
    this.requestLimiter = requestLimiter;
    this.traceWriter =
        new TraceWriter(
            Paths.get(endpointConfiguration.tracesOutputPath()),
            endpointConfiguration.traceCompression());
    this.l1L2BridgeSharedConfiguration = lineaL1L2BridgeSharedConfiguration;
    this.traceFileVersion = endpointConfiguration.traceFileVersion();
    this.traceFileCaching = endpointConfiguration.caching();
    // log configuration
    log.info("trace file caching {}", this.traceFileCaching ? "enabled." : "disabled.");
    log.info(
        "trace file compression {}",
        endpointConfiguration.traceCompression() ? "enabled." : "disabled.");
    log.info("trace file format is v{}.", this.traceFileVersion);
  }

  public String getNamespace() {
    return "linea";
  }

  public String getName() {
    return "generateConflatedTracesToFileV2";
  }

  /**
   * Handles execution traces generation logic.
   *
   * @param request holds parameters of the RPC request.
   * @return an execution file trace.
   */
  public TraceFile execute(final PluginRpcRequest request) {
    return requestLimiter.execute(request, this::generateTraceFile);
  }

  private TraceFile generateTraceFile(PluginRpcRequest request) {
    final Stopwatch sw = Stopwatch.createStarted();

    this.traceService =
        Optional.ofNullable(traceService).orElse(BesuServiceProvider.getTraceService(besuContext));

    final Object[] rawParams = request.getParams();

    Validator.validatePluginRpcRequestParams(rawParams);

    final TraceRequestParams params =
        CONVERTER.fromJson(CONVERTER.toJson(rawParams[0]), TraceRequestParams.class);

    params.validate();

    final long fromBlock = params.startBlockNumber();
    final long toBlock = params.endBlockNumber();
    // Determine expected path of the trace file.
    Path path =
        this.traceWriter.traceFilePath(
            fromBlock,
            toBlock,
            params.expectedTracesEngineVersion(),
            TraceRequestParams.getBesuRuntime());
    // Check whether the trace file already exists (or not).
    if (cachedTraceFileAvailable(path)) {
      log.info("[TRACING] cached trace for {}-{} detected as {}", fromBlock, toBlock, path);
    } else {
      final BlockchainService blockchainService =
          BesuServiceProvider.getBesuService(besuContext, BlockchainService.class);
      final Fork fork = getForkFromBesuBlockchainService(blockchainService, fromBlock, toBlock);
      final BigInteger chainId =
          blockchainService
              .getChainId()
              .orElseThrow(() -> new IllegalStateException("ChainId must be provided"));
      final PublicInputs publicInputs = generatePublicInputs(blockchainService, fromBlock, toBlock);

      final ZkTracer tracer =
          new ZkTracer(fork, l1L2BridgeSharedConfiguration, chainId, publicInputs);
      // Configure trace file version
      tracer.setLtFileMajorVersion(traceFileVersion);
      // Run the tracer
      traceService.trace(
          fromBlock,
          toBlock,
          worldStateBeforeTracing -> tracer.traceStartConflation(toBlock - fromBlock + 1),
          tracer::traceEndConflation,
          tracer);

      log.info(
          "[TRACING] trace on fork {} for blocks {}-{} computed in {}",
          fork,
          fromBlock,
          toBlock,
          sw);
      sw.reset().start();
      // Generate trace file
      path =
          traceWriter.writeTraceToFile(
              tracer,
              params.startBlockNumber(),
              params.endBlockNumber(),
              params.expectedTracesEngineVersion(),
              TraceRequestParams.getBesuRuntime());
      log.info("[TRACING] trace for {}-{} serialized to {} in {}", fromBlock, toBlock, path, sw);
    }

    return new TraceFile(params.expectedTracesEngineVersion(), path.toString());
  }

  /**
   * Determine whether a suitable trace file already exists in the desired location, and that it has
   * the correction versioning, etc.
   *
   * @param path Expected path for tracefile
   * @return
   */
  private boolean cachedTraceFileAvailable(final Path path) {
    // Initial sanity checks
    if (!Files.exists(path)) {
      // trace file doesn't exist.
      return false;
    } else if (!traceFileCaching) {
      // Caching disabled.
      log.info("[TRACING] cached trace {} ignored (caching disabled)", path);
      return false;
    }
    //
    return true;
  }
}
