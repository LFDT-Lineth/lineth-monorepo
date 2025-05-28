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

import static com.fasterxml.jackson.annotation.JsonInclude.Include.NON_ABSENT;

import java.util.List;
import java.util.Optional;

import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.annotation.JsonPropertyOrder;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.parameters.UnsignedLongParameter;

@JsonInclude(NON_ABSENT)
@JsonPropertyOrder({"blockNumber", "minTimestamp", "maxTimestamp"})
public record BundleParameter(
    /*  array of signed transactions to execute in a bundle */
    List<String> txs,
    /* block number for which this bundle is valid */
    Long blockNumber,
    /* Optional minimum timestamp from which this bundle is valid */
    Optional<Long> minTimestamp,
    /* Optional max timestamp for which this bundle is valid */
    Optional<Long> maxTimestamp,
    /* Optional list of transaction hashes which are allowed to revert */
    Optional<List<Hash>> revertingTxHashes,
    /* Optional UUID which can be used to replace or cancel this bundle */
    Optional<String> replacementUUID,
    /* Optional list of builders to share this bundle with */
    Optional<List<String>> builders) {
  @JsonCreator
  public BundleParameter(
      @JsonProperty("txs") final List<String> txs,
      @JsonProperty("blockNumber") final UnsignedLongParameter blockNumber,
      @JsonProperty("minTimestamp") final Optional<Long> minTimestamp,
      @JsonProperty("maxTimestamp") final Optional<Long> maxTimestamp,
      @JsonProperty("revertingTxHashes") final Optional<List<Hash>> revertingTxHashes,
      @JsonProperty("replacementUUID") final Optional<String> replacementUUID,
      @JsonProperty("builders") final Optional<List<String>> builders) {
    this(
        txs,
        blockNumber.getValue(),
        minTimestamp,
        maxTimestamp,
        revertingTxHashes,
        replacementUUID,
        builders);
  }
}
