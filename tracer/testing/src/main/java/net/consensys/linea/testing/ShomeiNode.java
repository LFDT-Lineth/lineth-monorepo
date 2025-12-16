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

package net.consensys.linea.testing;

import com.fasterxml.jackson.databind.node.ArrayNode;
import java.nio.file.Path;
import lombok.extern.slf4j.Slf4j;
import net.consensys.shomei.Runner;
import net.consensys.shomei.cli.option.DataStorageOption;
import net.consensys.shomei.cli.option.HashFunctionOption;
import net.consensys.shomei.cli.option.JsonRpcOption;
import net.consensys.shomei.cli.option.MetricsOption;
import net.consensys.shomei.cli.option.SyncOption;

@Slf4j
public class ShomeiNode extends Runner implements AutoCloseable, Runnable {

  public record MerkelProofResponse(
      String zkParentStateRootHash,
      String zkEndStateRootHash,
      ArrayNode zkStateMerkleProof,
      String zkStateManagerVersion) {}

  private String jsonRpcUrl;

  public ShomeiNode(
      DataStorageOption dataStorageOption,
      JsonRpcOption jsonRpcOption,
      SyncOption syncOption,
      MetricsOption metricsOption,
      HashFunctionOption hashFunctionOption) {
    super(dataStorageOption, jsonRpcOption, syncOption, metricsOption, hashFunctionOption);
    this.jsonRpcUrl =
        "http://" + jsonRpcOption.getRpcHttpHost() + ":" + jsonRpcOption.getRpcHttpPort();
  }

  public String getJsonRpcUrl() {
    return this.jsonRpcUrl;
  }

  @Override
  public void run() {
    super.start();
  }

  @Override
  public void close() {
    try {
      super.stop();
    } catch (Exception e) {
      log.error("Error stopping Shomei node", e);
    }
  }

  public static class Builder {
    private final DataStorageOption.Builder dataStorageOptionBuilder =
        new DataStorageOption.Builder();
    private final JsonRpcOption.Builder jsonRpcOptionBuilder = new JsonRpcOption.Builder();
    private final SyncOption.Builder syncOptionBuilder = new SyncOption.Builder();

    public Builder setDataStoragePath(Path dataStoragePath) {
      this.dataStorageOptionBuilder.setDataStoragePath(dataStoragePath);
      return this;
    }

    public Builder setJsonRpcPort(Integer port) {
      jsonRpcOptionBuilder.setRpcHttpHost("127.0.0.1").setRpcHttpPort(port);
      return this;
    }

    public Builder setBesuRpcPort(Integer port) {
      jsonRpcOptionBuilder.setBesuRpcHttpHost("127.0.0.1").setBesuHttpPort(port);
      return this;
    }

    public ShomeiNode build() {
      return new ShomeiNode(
          dataStorageOptionBuilder.build(),
          jsonRpcOptionBuilder.build(),
          syncOptionBuilder.build(),
          new MetricsOption.Builder().setEnableMetrics(false).build(),
          new HashFunctionOption.Builder()
              .setHashFunction(HashFunctionOption.HashFunction.MIMC_BLS12_377)
              .build());
    }
  }

  public record GetZkEVMStateMerkleProofResponse(
      ArrayNode zkStateMerkleProof,
      byte[] zkParentStateRootHash,
      byte[] zkEndStateRootHash,
      String zkStateManagerVersion) {
    public static GetZkEVMStateMerkleProofResponse fromJson(String json) {
      // Implement JSON deserialization logic here
      return null; // Placeholder, replace with actual deserialization
    }
  }
}
