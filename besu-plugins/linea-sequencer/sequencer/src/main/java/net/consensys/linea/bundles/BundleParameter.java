/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.bundles;

import static com.fasterxml.jackson.annotation.JsonInclude.Include.NON_ABSENT;

import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.annotation.JsonPropertyOrder;
import java.util.List;
import java.util.Optional;
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
