/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package lineth.sequencer.tracing;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatIllegalArgumentException;

import java.util.OptionalLong;
import org.junit.jupiter.api.Test;

class BespokeTracingActivationPolicyTest {
  @Test
  void shouldAlwaysTraceWhenCutoffIsUnset() {
    final var policy = new BespokeTracingActivationPolicy(OptionalLong.empty());

    assertThat(policy.shouldTrace(Long.MAX_VALUE)).isTrue();
  }

  @Test
  void shouldTraceOnlyBeforeCutoff() {
    final var policy = new BespokeTracingActivationPolicy(OptionalLong.of(100L));

    assertThat(policy.shouldTrace(99L)).isTrue();
    assertThat(policy.shouldTrace(100L)).isFalse();
    assertThat(policy.shouldTrace(101L)).isFalse();
  }

  @Test
  void shouldRejectNegativeCutoff() {
    assertThatIllegalArgumentException()
        .isThrownBy(() -> new BespokeTracingActivationPolicy(OptionalLong.of(-1L)))
        .withMessage("Tracing end timestamp must be zero or greater");
  }
}
