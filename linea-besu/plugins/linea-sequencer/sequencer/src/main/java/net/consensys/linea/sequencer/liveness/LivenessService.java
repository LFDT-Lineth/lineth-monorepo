/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.liveness;

import java.util.Optional;
import net.consensys.linea.bundles.TransactionBundle;

public interface LivenessService {
  Optional<TransactionBundle> checkBlockTimestampAndBuildBundle(
      long currentTimestamp, long lastBlockTimestamp, long targetBlockNumber);

  void updateUptimeMetrics(boolean isSucceeded, long blockTimestamp);
}
