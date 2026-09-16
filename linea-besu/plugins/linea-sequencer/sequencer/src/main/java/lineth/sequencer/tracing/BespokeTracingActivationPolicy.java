/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package lineth.sequencer.tracing;

import java.util.OptionalLong;

/** Determines whether bespoke tracing is active for a pending block. */
public record BespokeTracingActivationPolicy(OptionalLong tracingEndTimestamp) {
  public BespokeTracingActivationPolicy {
    if (tracingEndTimestamp == null) {
      tracingEndTimestamp = OptionalLong.empty();
    }
    if (tracingEndTimestamp.isPresent() && tracingEndTimestamp.getAsLong() < 0) {
      throw new IllegalArgumentException("Tracing end timestamp must be zero or greater");
    }
  }

  public boolean shouldTrace(final long pendingBlockTimestamp) {
    return tracingEndTimestamp.isEmpty() || pendingBlockTimestamp < tracingEndTimestamp.getAsLong();
  }
}
