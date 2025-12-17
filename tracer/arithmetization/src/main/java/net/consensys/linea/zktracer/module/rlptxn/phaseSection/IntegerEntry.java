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

package net.consensys.linea.zktracer.module.rlptxn.phaseSection;

public enum IntegerEntry {
  CHAIN_ID,
  NONCE,
  GAS_PRICE,
  MAX_PRIORITY_FEE_PER_GAS,
  MAX_FEE_PER_GAS,
  GAS_LIMIT,
  VALUE,
  Y,
  R,
  S;

  public boolean lx() {
    return this == CHAIN_ID
        || this == NONCE
        || this == GAS_PRICE
        || this == MAX_PRIORITY_FEE_PER_GAS
        || this == MAX_FEE_PER_GAS
        || this == GAS_LIMIT
        || this == VALUE;
  }
}
