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

package net.consensys.linea.jsonrpc;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.file.DirectoryStream;
import java.nio.file.FileAlreadyExistsException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.nio.file.StandardOpenOption;
import java.time.Duration;
import java.time.Instant;
import java.util.Comparator;
import java.util.Map;
import java.util.TreeSet;
import java.util.UUID;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CompletionException;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.annotations.VisibleForTesting;
import lombok.NonNull;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.config.LineaNodeType;
import net.consensys.linea.config.LineaRejectedTxReportingConfiguration;
import okhttp3.MediaType;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.RequestBody;
import okhttp3.Response;

/** This class is responsible for managing JSON-RPC requests for reporting rejected transactions */
@Slf4j
public class JsonRpcManager {
  private static final Duration INITIAL_RETRY_DELAY_DURATION = Duration.ofSeconds(1);
  private static final Duration MAX_RETRY_DURATION = Duration.ofHours(2);
  private static final MediaType JSON = MediaType.get("application/json; charset=utf-8");
  static final String JSON_RPC_DIR = "rej-tx-rpc";
  static final String DISCARDED_DIR = "discarded";

  private final OkHttpClient client = new OkHttpClient();
  private final ObjectMapper objectMapper = new ObjectMapper();
  private final Map<Path, Instant> fileStartTimes = new ConcurrentHashMap<>();

  private final Path jsonRpcDir;
  private final LineaRejectedTxReportingConfiguration reportingConfiguration;
  private final ExecutorService executorService;
  private final ScheduledExecutorService retrySchedulerService;

  /**
   * Creates a new JSON-RPC manager.
   *
   * @param pluginIdentifier The plugin identifier will be created as a sub-directory under
   *     rej-tx-rpc. The rejected transactions will be stored under it for each plugin that uses it.
   * @param besuDataDir Path to Besu data directory. The json-rpc files will be stored here under
   *     rej-tx-rpc subdirectory.
   * @param reportingConfiguration Instance of LineaRejectedTxReportingConfiguration containing the
   *     endpoint URI and node type.
   */
  public JsonRpcManager(
      @NonNull final String pluginIdentifier,
      @NonNull final Path besuDataDir,
      @NonNull final LineaRejectedTxReportingConfiguration reportingConfiguration) {
    if (reportingConfiguration.rejectedTxEndpoint() == null) {
      throw new IllegalStateException("Rejected transaction endpoint URI is required");
    }
    this.jsonRpcDir = besuDataDir.resolve(JSON_RPC_DIR).resolve(pluginIdentifier);
    this.reportingConfiguration = reportingConfiguration;
    this.executorService = Executors.newVirtualThreadPerTaskExecutor();
    this.retrySchedulerService = Executors.newSingleThreadScheduledExecutor();
  }

  /** Load existing JSON-RPC and submit them. */
  public JsonRpcManager start() {
    try {
      // Create the rej-tx-rpc/pluginIdentifier/discarded directories if it doesn't exist
      Files.createDirectories(jsonRpcDir.resolve(DISCARDED_DIR));

      // Load existing JSON files
      processExistingJsonFiles();
      return this;
    } catch (final IOException e) {
      log.error("Failed to create or access directories under: {}", jsonRpcDir, e);
      throw new UncheckedIOException(e);
    }
  }

  /** Shuts down the executor service and scheduler service. */
  public void shutdown() {
    executorService.shutdown();
    retrySchedulerService.shutdown();
  }

  /**
   * Submits a new JSON-RPC call.
   *
   * @param jsonContent The JSON content to submit
   */
  public void submitNewJsonRpcCallAsync(final String jsonContent) {
    CompletableFuture.supplyAsync(
            () -> {
              try {
                Path jsonFile = saveJsonToDir(jsonContent, jsonRpcDir);
                fileStartTimes.put(jsonFile, Instant.now());
                return jsonFile;
              } catch (final IOException e) {
                log.error("Failed to save JSON-RPC content", e);
                throw new CompletionException(e);
              }
            },
            executorService)
        .thenAcceptAsync(
            jsonFile -> submitJsonRpcCall(jsonFile, INITIAL_RETRY_DELAY_DURATION), executorService)
        .exceptionally(
            e -> {
              log.error("Error in submitNewJsonRpcCall", e);
              return null;
            });
  }

  public LineaNodeType getNodeType() {
    return reportingConfiguration.lineaNodeType();
  }

  private void processExistingJsonFiles() {
    try {
      final TreeSet<Path> sortedFiles = new TreeSet<>(Comparator.comparing(Path::getFileName));

      try (DirectoryStream<Path> stream = Files.newDirectoryStream(jsonRpcDir, "rpc_*.json")) {
        for (Path path : stream) {
          sortedFiles.add(path);
        }
      }

      log.info("Loaded {} existing JSON-RPC files for reporting", sortedFiles.size());

      for (Path path : sortedFiles) {
        fileStartTimes.put(path, Instant.now());
        submitJsonRpcCall(path, INITIAL_RETRY_DELAY_DURATION);
      }
    } catch (final IOException e) {
      log.error("Failed to load existing JSON-RPC files", e);
    }
  }

  private void submitJsonRpcCall(final Path jsonFile, final Duration nextDelay) {
    executorService.submit(
        () -> {
          if (!Files.exists(jsonFile)) {
            log.debug("JSON-RPC file {} no longer exists, skipping processing.", jsonFile);
            fileStartTimes.remove(jsonFile);
            return;
          }
          try {
            final String jsonContent = new String(Files.readAllBytes(jsonFile));
            final boolean success = sendJsonRpcCall(jsonContent);
            if (success) {
              Files.deleteIfExists(jsonFile);
              fileStartTimes.remove(jsonFile);
            } else {
              log.error(
                  "Failed to send JSON-RPC file {} to {}, Scheduling retry ...",
                  jsonFile,
                  reportingConfiguration.rejectedTxEndpoint());
              scheduleRetry(jsonFile, nextDelay);
            }
          } catch (final Exception e) {
            log.error(
                "Failed to process JSON-RPC file {} due to unexpected error: {}. Scheduling retry ...",
                jsonFile,
                e.getMessage());
            scheduleRetry(jsonFile, nextDelay);
          }
        });
  }

  private void scheduleRetry(final Path jsonFile, final Duration currentDelay) {
    final Instant startTime = fileStartTimes.get(jsonFile);
    if (startTime == null) {
      log.debug("No start time found for JSON-RPC file: {}. Skipping retry.", jsonFile);
      return;
    }

    // Check if we're still within the maximum retry duration
    if (Duration.between(startTime, Instant.now()).compareTo(MAX_RETRY_DURATION) < 0) {
      // Calculate next delay with exponential backoff, capped at 1 minute
      final Duration nextDelay =
          Duration.ofMillis(
              Math.min(currentDelay.multipliedBy(2).toMillis(), Duration.ofMinutes(1).toMillis()));

      // Schedule a retry
      retrySchedulerService.schedule(
          () -> submitJsonRpcCall(jsonFile, nextDelay),
          currentDelay.toMillis(),
          TimeUnit.MILLISECONDS);
    } else {
      log.error("Exceeded maximum retry duration for JSON-RPC file: {}.", jsonFile);
      final Path destination = jsonRpcDir.resolve(DISCARDED_DIR).resolve(jsonFile.getFileName());

      try {
        Files.move(jsonFile, destination, StandardCopyOption.REPLACE_EXISTING);
        log.error(
            "The JSON-RPC file {} has been moved to: {}. The tx notification has been discarded.",
            jsonFile,
            destination);
      } catch (final IOException e) {
        log.error("Failed to move JSON-RPC file to discarded directory: {}", jsonFile, e);
      } finally {
        fileStartTimes.remove(jsonFile);
      }
    }
  }

  private boolean sendJsonRpcCall(final String jsonContent) {
    final RequestBody body = RequestBody.create(jsonContent, JSON);
    final Request request =
        new Request.Builder().url(reportingConfiguration.rejectedTxEndpoint()).post(body).build();

    try (final Response response = client.newCall(request).execute()) {
      if (!response.isSuccessful()) {
        log.error("Unexpected response code from rejected-tx endpoint: {}", response.code());
        return false;
      }

      // process the response body here ...
      final String responseBody = response.body() != null ? response.body().string() : null;
      if (responseBody == null) {
        log.error("Unexpected empty response body from rejected-tx endpoint");
        return false;
      }

      final JsonNode jsonNode = objectMapper.readTree(responseBody);
      if (jsonNode == null) {
        log.error("Failed to parse JSON response from rejected-tx endpoint: {}", responseBody);
        return false;
      }
      if (jsonNode.has("error")) {
        log.error("Error response from rejected-tx endpoint: {}", jsonNode.get("error"));
        return false;
      }
      // Check for result
      if (jsonNode.has("result")) {
        final String status = jsonNode.get("result").get("status").asText();
        log.debug("Rejected-tx JSON-RPC call successful. Status: {}", status);
        return true;
      }

      log.warn("Unexpected rejected-tx JSON-RPC response format: {}", responseBody);
      return false;
    } catch (final IOException e) {
      log.error(
          "Failed to send JSON-RPC call to rejected-tx endpoint {}",
          reportingConfiguration.rejectedTxEndpoint(),
          e);
      return false;
    }
  }

  /**
   * Saves the given JSON content to a file in the rejected transactions RPC directory. The filename
   * is generated using a high-precision timestamp and a UUID to ensure uniqueness.
   *
   * <p>The file naming format is: rpc_[timestamp]_[uuid].json
   *
   * @param jsonContent The JSON string to be written to the file.
   * @param rejTxRpcDirectory The directory where the file should be saved.
   * @return The Path object representing the newly created file.
   * @throws IOException If an I/O error occurs while writing the file, including unexpected file
   *     collisions.
   */
  @VisibleForTesting
  static Path saveJsonToDir(final String jsonContent, final Path rejTxRpcDirectory)
      throws IOException {
    final String timestamp = generateTimestampWithNanos();
    final String uuid = UUID.randomUUID().toString();
    final String fileName = String.format("rpc_%s_%s.json", timestamp, uuid);
    final Path filePath = rejTxRpcDirectory.resolve(fileName);

    try {
      return Files.writeString(filePath, jsonContent, StandardOpenOption.CREATE_NEW);
    } catch (final FileAlreadyExistsException e) {
      // This should never happen with UUID, but just in case
      log.warn("Unexpected JSON-RPC filename collision occurred: {}", filePath);
      throw new IOException("Unexpected file name collision", e);
    }
  }

  static String generateTimestampWithNanos() {
    final Instant now = Instant.now();
    final long seconds = now.getEpochSecond();
    final int nanos = now.getNano();
    return String.format("%d%09d", seconds, nanos);
  }
}
