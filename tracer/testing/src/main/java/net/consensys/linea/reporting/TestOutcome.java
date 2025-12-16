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
package net.consensys.linea.reporting;

import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.annotation.JsonPropertyOrder;
import java.util.concurrent.ConcurrentMap;
import java.util.concurrent.ConcurrentSkipListSet;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

@Getter
@Setter
@NoArgsConstructor(force = true)
@AllArgsConstructor
@JsonPropertyOrder({
  "failedCounter",
  "successCounter",
  "disabledCounter",
  "abortedCounter",
  "modulesToConstraintsToTests"
})
public class TestOutcome {
  @JsonProperty private final int failedCounter;
  @JsonProperty private final int successCounter;
  @JsonProperty private final int disabledCounter;
  @JsonProperty private final int abortedCounter;

  @JsonProperty
  private final ConcurrentMap<String, ConcurrentMap<String, ConcurrentSkipListSet<String>>>
      modulesToConstraintsToTests;
}
