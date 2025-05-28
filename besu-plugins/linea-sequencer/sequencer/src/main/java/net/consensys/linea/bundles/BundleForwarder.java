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
package net.consensys.linea.bundles;

import static com.fasterxml.jackson.annotation.JsonAutoDetect.Visibility.ANY;

import java.io.IOException;
import java.net.URL;
import java.util.OptionalLong;
import java.util.concurrent.Callable;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicLong;

import com.fasterxml.jackson.annotation.JsonAutoDetect;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jdk8.Jdk8Module;
import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.ToString;
import lombok.experimental.Accessors;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.bundles.BundlePoolService.TransactionBundleAddedListener;
import net.consensys.linea.bundles.BundlePoolService.TransactionBundleRemovedListener;
import net.consensys.linea.config.LineaBundleConfiguration;
import net.consensys.linea.utils.PriorityThreadPoolExecutor;
import okhttp3.MediaType;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.RequestBody;
import okhttp3.Response;
import org.hyperledger.besu.plugin.services.BlockchainService;

@Slf4j
@RequiredArgsConstructor
public class BundleForwarder
    implements TransactionBundleAddedListener, TransactionBundleRemovedListener {
  public static final String RETRY_COUNT_HEADER = "X-Retry-Count";
  private final AtomicLong reqIdProvider = new AtomicLong(0L);
  private final LineaBundleConfiguration config;
  private final PriorityThreadPoolExecutor executor;
  private final ScheduledExecutorService retryScheduler;
  private final BlockchainService blockchainService;
  private final OkHttpClient rpcClient;
  private final URL recipientUrl;

  @Override
  public void onTransactionBundleAdded(final TransactionBundle bundle) {
    executor.submit(new SendBundleTask(bundle, 0));
  }

  @Override
  public void onTransactionBundleRemoved(final TransactionBundle transactionBundle) {
    executor.remove(new SendBundleTask(transactionBundle, 0));
  }

  void retry(final TransactionBundle bundle, final int retry) {
    retryScheduler.schedule(
        () -> executor.submit(new SendBundleTask(bundle, retry)),
        config.retryDelayMillis(),
        TimeUnit.MILLISECONDS);
  }

  @RequiredArgsConstructor
  @EqualsAndHashCode(onlyExplicitlyIncluded = true)
  class SendBundleTask implements Callable<SendBundleResponse>, Comparable<SendBundleTask> {
    private static final ObjectMapper OBJECT_MAPPER =
        new ObjectMapper().registerModule(new Jdk8Module());
    private static final MediaType JSON = MediaType.get("application/json; charset=utf-8");
    @Getter @EqualsAndHashCode.Include private final TransactionBundle bundle;
    private final int retryCount;

    @Override
    public SendBundleResponse call() throws BundleForwarderException {
      final var chainHeadBlockNumber = blockchainService.getChainHeadHeader().getNumber();
      if (bundle.blockNumber() <= chainHeadBlockNumber) {
        throw new BundleForwarderException(
            "Skip forwarding bundle for past block number "
                + bundle.blockNumber()
                + " since chain head block number is "
                + chainHeadBlockNumber,
            bundle);
      }

      final long reqId = reqIdProvider.getAndIncrement();
      final var jsonRpcRequest = new JsonRpcEnvelope(reqId, bundle.toBundleParameter(false));

      log.trace("Forwarding request {}, retry count {}", jsonRpcRequest, retryCount);

      final RequestBody body;
      try {
        body = RequestBody.create(OBJECT_MAPPER.writeValueAsString(jsonRpcRequest), JSON);
      } catch (JsonProcessingException e) {
        log.error("Error creating send bundle request body", e);
        throw new BundleForwarderException(
            "Error creating send bundle request body", e, bundle, reqId);
      }

      final var requestBuilder = new Request.Builder().url(recipientUrl).post(body);

      if (retryCount > 0) {
        requestBuilder.addHeader(RETRY_COUNT_HEADER, String.valueOf(retryCount));
      }

      try (final Response response = rpcClient.newCall(requestBuilder.build()).execute()) {
        final var result =
            new SendBundleResponse(reqId, bundle, response, response.body().string());
        if (response.isSuccessful()) {
          log.trace(
              "Bundle {} forwarded successfully at retry count {}", jsonRpcRequest, retryCount);
        } else {
          log.error(
              "Bundle {} forward failed with status {} at retry count {}",
              jsonRpcRequest,
              response.code(),
              retryCount);
        }
        return result;
      } catch (IOException e) {
        log.warn(
            "Error forwarding bundle request {}, at retry count {}, retrying later",
            jsonRpcRequest,
            retryCount,
            e);
        retry(bundle, retryCount + 1);
        throw new BundleForwarderException(
            "Error send bundle request, retrying later", e, bundle, reqId);
      }
    }

    @Override
    public int compareTo(final SendBundleTask o) {
      final int blockNumberPlusRetriesComp =
          Long.compare(this.blockNumberPlusRetries(), o.blockNumberPlusRetries());
      if (blockNumberPlusRetriesComp == 0) {
        // put retries at the end
        final int retryCountComp = Integer.compare(this.retryCount, o.retryCount);
        if (retryCountComp == 0) {
          // at last disambiguate by sequence
          return Long.compare(this.bundle.sequence(), o.bundle.sequence());
        }
        return retryCountComp;
      }
      return blockNumberPlusRetriesComp;
    }

    private long blockNumberPlusRetries() {
      return this.bundle.blockNumber() + retryCount;
    }
  }

  record SendBundleResponse(long reqId, TransactionBundle bundle, Response response, String body) {}

  @JsonAutoDetect(fieldVisibility = ANY)
  @ToString(onlyExplicitlyIncluded = true)
  private static class JsonRpcEnvelope {
    private final String jsonrpc = "2.0";
    private final String method = "linea_sendBundle";
    @ToString.Include private final long id;
    @ToString.Include private final BundleParameter[] params;

    public JsonRpcEnvelope(final long id, final BundleParameter params) {
      this.id = id;
      this.params = new BundleParameter[] {params};
    }
  }

  @Accessors(fluent = true)
  @Getter
  public class BundleForwarderException extends RuntimeException {
    private final OptionalLong reqId;
    private final TransactionBundle bundle;

    public BundleForwarderException(final String message, final TransactionBundle bundle) {
      super(message);
      this.reqId = OptionalLong.empty();
      this.bundle = bundle;
    }

    public BundleForwarderException(
        final String message,
        final Throwable cause,
        final TransactionBundle bundle,
        final long reqId) {
      super(message, cause);
      this.reqId = OptionalLong.of(reqId);
      this.bundle = bundle;
    }

    @Override
    public String getMessage() {
      return "[ReqId:" + reqId + "] " + super.getMessage();
    }
  }
}
