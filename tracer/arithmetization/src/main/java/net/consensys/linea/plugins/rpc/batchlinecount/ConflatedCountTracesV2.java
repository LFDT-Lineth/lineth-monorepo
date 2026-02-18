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

package net.consensys.linea.plugins.rpc.batchlinecount;

import static net.consensys.linea.plugins.rpc.LineCounterGenerator.createLineCountingTracer;

import com.google.common.base.Stopwatch;
import java.util.Map;
import java.util.Optional;
import java.util.TreeMap;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.plugins.BesuServiceProvider;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.plugins.config.LineaTracerSharedConfiguration;
import net.consensys.linea.plugins.rpc.RequestLimiter;
import net.consensys.linea.plugins.rpc.Validator;
import net.consensys.linea.plugins.rpc.tracegeneration.TraceRequestParams;
import net.consensys.linea.zktracer.LineCountingTracer;
import net.consensys.linea.zktracer.json.JsonConverter;
import org.hyperledger.besu.plugin.ServiceManager;
import org.hyperledger.besu.plugin.services.TraceService;
import org.hyperledger.besu.plugin.services.rpc.PluginRpcRequest;

/**
 * Sets up an RPC endpoint for generating conflated file trace. This class provides an RPC endpoint
 * named 'generateConflatedTracesToFileV0' under the 'rollup' namespace. When this endpoint is
 * called, it triggers the execution of the 'execute' method, which generates conflated file traces
 * based on the provided request parameters and writes them to a file.
 */
@Slf4j
@RequiredArgsConstructor
public class ConflatedCountTracesV2 {
  private static final JsonConverter CONVERTER = JsonConverter.builder().build();
  private final RequestLimiter requestLimiter;
  private final ServiceManager besuContext;
  private final LineaL1L2BridgeSharedConfiguration l1L2BridgeSharedConfiguration;
  private final LineaTracerSharedConfiguration tracerSharedConfiguration;
  private TraceService traceService;

  public String getNamespace() {
    return "linea";
  }

  public String getName() {
    return "getConflatedTracesCountersV2";
  }

  /**
   * Handles execution traces generation logic.
   *
   * @param request holds parameters of the RPC request.
   * @return an execution file trace.
   */
  public ConflatedLineCounts execute(final PluginRpcRequest request) {
    return requestLimiter.execute(request, this::countConflation);
  }

  private ConflatedLineCounts countConflation(PluginRpcRequest request) {
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
    final LineCountingTracer counter =
        createLineCountingTracer(
            besuContext,
            tracerSharedConfiguration,
            l1L2BridgeSharedConfiguration,
            fromBlock,
            toBlock);

    traceService.trace(
        fromBlock,
        toBlock,
        worldStateBeforeTracing -> counter.traceStartConflation(toBlock - fromBlock + 1),
        counter::traceEndConflation,
        counter);

    final Map<String, Integer> counts = counter.getModulesLineCount();
    log.info(
        "[TRACING] counting lines for conflated blocks {}-{} computed in {}",
        fromBlock,
        toBlock,
        sw);
    sw.reset().start();

    return new ConflatedLineCounts(
        TraceRequestParams.getTracerRuntime(), fromBlock, toBlock, new TreeMap<>(counts));
  }
}
