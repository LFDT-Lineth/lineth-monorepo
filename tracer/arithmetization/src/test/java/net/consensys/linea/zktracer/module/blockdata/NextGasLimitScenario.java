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
package net.consensys.linea.zktracer.module.blockdata;

public enum NextGasLimitScenario {
  IN_RANGE_SAME,
  IN_RANGE_INCREMENT,
  IN_RANGE_DECREMENT,
  IN_RANGE_MAX,
  IN_RANGE_MIN,
  OUT_OF_RANGE_INCREMENT,
  OUT_OF_RANGE_DECREMENT,
  OUT_OF_RANGE_GENERIC;

  public boolean isInRange() {
    return this == IN_RANGE_SAME || this == IN_RANGE_INCREMENT || this == IN_RANGE_DECREMENT;
  }

  public boolean isOutOfRange() {
    return this == OUT_OF_RANGE_INCREMENT
        || this == OUT_OF_RANGE_DECREMENT
        || this == IN_RANGE_MAX
        || this == IN_RANGE_MIN;
  }
}
