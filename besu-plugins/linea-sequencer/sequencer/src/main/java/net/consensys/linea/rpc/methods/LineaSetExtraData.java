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

package net.consensys.linea.rpc.methods;

import static net.consensys.linea.extradata.LineaExtraDataException.ErrorType.FAILED_CALLING_SET_EXTRA_DATA;
import static net.consensys.linea.extradata.LineaExtraDataException.ErrorType.INVALID_ARGUMENT;

import java.util.concurrent.atomic.AtomicInteger;

import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.extradata.LineaExtraDataException;
import net.consensys.linea.extradata.LineaExtraDataHandler;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.ethereum.api.jsonrpc.internal.parameters.JsonRpcParameter;
import org.hyperledger.besu.plugin.services.RpcEndpointService;
import org.hyperledger.besu.plugin.services.exception.PluginRpcEndpointException;
import org.hyperledger.besu.plugin.services.rpc.PluginRpcRequest;
import org.hyperledger.besu.plugin.services.rpc.RpcMethodError;
import org.hyperledger.besu.plugin.services.rpc.RpcResponseType;

@Slf4j
public class LineaSetExtraData {

  private static final AtomicInteger LOG_SEQUENCE = new AtomicInteger();
  private final JsonRpcParameter parameterParser = new JsonRpcParameter();
  private final RpcEndpointService rpcEndpointService;
  private LineaExtraDataHandler extraDataHandler;

  public LineaSetExtraData(final RpcEndpointService rpcEndpointService) {
    this.rpcEndpointService = rpcEndpointService;
  }

  public void init(final LineaExtraDataHandler extraDataHandler) {
    this.extraDataHandler = extraDataHandler;
  }

  public String getNamespace() {
    return "linea";
  }

  public String getName() {
    return "setExtraData";
  }

  public Boolean execute(final PluginRpcRequest request) {
    // no matter if it overflows, since it is only used to correlate logs for this request,
    // so we only print callParameters once at the beginning, and we can reference them using the
    // sequence.
    final int logId = log.isDebugEnabled() ? LOG_SEQUENCE.incrementAndGet() : -1;

    try {
      final var extraData = parseRequest(logId, request.getParams());

      updatePricingConf(logId, extraData);

      updateStandardExtraData(extraData);

      return Boolean.TRUE;
    } catch (final LineaExtraDataException lede) {
      throw new PluginRpcEndpointException(new ExtraDataPricingError(lede));
    }
  }

  private void updateStandardExtraData(final Bytes32 extraData) {
    final var resp =
        rpcEndpointService.call("miner_setExtraData", new Object[] {extraData.toHexString()});
    if (!resp.getType().equals(RpcResponseType.SUCCESS)) {
      throw new LineaExtraDataException(
          FAILED_CALLING_SET_EXTRA_DATA, "Internal setExtraData method failed: " + resp);
    }
  }

  private void updatePricingConf(final int logId, final Bytes32 extraData) {
    extraDataHandler.handle(extraData);
    log.atDebug()
        .setMessage("[{}] Successfully handled extra data pricing")
        .addArgument(logId)
        .log();
  }

  private Bytes32 parseRequest(final int logId, final Object[] params) {
    try {
      final var rawParam = parameterParser.required(params, 0, String.class);
      final var extraData = Bytes32.wrap(Bytes.fromHexStringLenient(rawParam));
      log.atDebug()
          .setMessage("[{}] set extra data, raw=[{}] parsed=[{}]")
          .addArgument(logId)
          .addArgument(rawParam)
          .addArgument(extraData::toHexString)
          .log();
      return extraData;
    } catch (Exception e) {
      throw new LineaExtraDataException(INVALID_ARGUMENT, e.getMessage());
    }
  }

  private record ExtraDataPricingError(LineaExtraDataException ex) implements RpcMethodError {
    @Override
    public int getCode() {
      return ex.getErrorType().getCode();
    }

    @Override
    public String getMessage() {
      return ex.getMessage();
    }
  }
}
