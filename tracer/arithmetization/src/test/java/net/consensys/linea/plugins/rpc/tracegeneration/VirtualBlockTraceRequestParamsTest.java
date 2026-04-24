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

package net.consensys.linea.plugins.rpc.tracegeneration;

import static org.assertj.core.api.Assertions.assertThatNoException;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import org.junit.jupiter.api.Test;

class VirtualBlockTraceRequestParamsTest {

  @Test
  void validParamsPassValidation() {
    VirtualBlockTraceRequestParams params =
        new VirtualBlockTraceRequestParams(100L, new String[] {"0xf86c0a8502540be400825208..."});

    assertThatNoException().isThrownBy(params::validate);
  }

  @Test
  void blockNumberZeroThrowsException() {
    VirtualBlockTraceRequestParams params =
        new VirtualBlockTraceRequestParams(0L, new String[] {"0xf86c..."});

    assertThatThrownBy(params::validate)
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("INVALID_BLOCK_NUMBER")
        .hasMessageContaining("must be at least 1");
  }

  @Test
  void negativeBlockNumberThrowsException() {
    VirtualBlockTraceRequestParams params =
        new VirtualBlockTraceRequestParams(-5L, new String[] {"0xf86c..."});

    assertThatThrownBy(params::validate)
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("INVALID_BLOCK_NUMBER");
  }

  @Test
  void nullTransactionsThrowsException() {
    VirtualBlockTraceRequestParams params = new VirtualBlockTraceRequestParams(100L, null);

    assertThatThrownBy(params::validate)
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("INVALID_TRANSACTIONS")
        .hasMessageContaining("must contain at least one transaction");
  }

  @Test
  void emptyTransactionsArrayThrowsException() {
    VirtualBlockTraceRequestParams params =
        new VirtualBlockTraceRequestParams(100L, new String[] {});

    assertThatThrownBy(params::validate)
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("INVALID_TRANSACTIONS")
        .hasMessageContaining("must contain at least one transaction");
  }

  @Test
  void validParamsWithMultipleTransactions() {
    VirtualBlockTraceRequestParams params =
        new VirtualBlockTraceRequestParams(
            100L, new String[] {"0xf86c0a...", "0xf86c0b...", "0xf86c0c..."});

    assertThatNoException().isThrownBy(params::validate);
  }

  @Test
  void blockNumberOneIsValid() {
    // Block number 1 is valid because parent block 0 (genesis) should exist
    VirtualBlockTraceRequestParams params =
        new VirtualBlockTraceRequestParams(1L, new String[] {"0xf86c..."});

    assertThatNoException().isThrownBy(params::validate);
  }
}
