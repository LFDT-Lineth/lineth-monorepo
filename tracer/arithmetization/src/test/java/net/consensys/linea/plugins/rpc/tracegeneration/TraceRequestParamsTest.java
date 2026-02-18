/*
 * Copyright ConsenSys Inc.
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

import static org.junit.jupiter.api.Assertions.*;

import net.consensys.linea.zktracer.json.JsonConverter;
import org.junit.jupiter.api.Test;

class TraceRequestParamsTest {

  private final JsonConverter jsonConverter = JsonConverter.builder().build();

  @Test
  void shouldParseValidParams() {
    assertEquals(
        new TraceRequestParams(10, 20),
        jsonConverter.fromJson(
            """
        {
          "startBlockNumber": 10,
          "endBlockNumber": 20
        }
        """,
            TraceRequestParams.class));

    assertEquals(
        new TraceRequestParams(10, 20),
        jsonConverter.fromJson(
            """
        {
          "startBlockNumber": 10,
          "endBlockNumber": 20,
          "expectedTracesEngineVersion": "test"
        }
        """,
            TraceRequestParams.class));
  }
}
