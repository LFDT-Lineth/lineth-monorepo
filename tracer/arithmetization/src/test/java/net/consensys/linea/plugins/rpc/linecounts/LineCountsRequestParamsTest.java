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

package net.consensys.linea.plugins.rpc.linecounts;

import static org.junit.jupiter.api.Assertions.*;

import net.consensys.linea.zktracer.json.JsonConverter;
import org.junit.jupiter.api.Test;

class LineCountsRequestParamsTest {

  private final JsonConverter jsonConverter = JsonConverter.builder().build();

  @Test
  void shouldParseValidParams() {
    assertEquals(
        new LineCountsRequestParams(12345L),
        jsonConverter.fromJson(
            """
      {
        "blockNumber": 12345
      }
      """,
            LineCountsRequestParams.class));

    assertEquals(
        new LineCountsRequestParams(12345L),
        jsonConverter.fromJson(
            """
      {
      "blockNumber": 12345,
      "expectedTracesEngineVersion": "test"
      }
      """,
            LineCountsRequestParams.class));
  }
}
