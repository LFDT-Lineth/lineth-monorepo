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

package net.consensys.linea.zktracer.module.hub.section;

public enum TupleAnalysis {
  TUPLE_FAILS_CHAIN_ID_CHECK,
  TUPLE_FAILS_NONCE_RANGE_CHECK, // the nonce refers to the tuple.nonce()
  TUPLE_FAILS_S_RANGE_CHECK,
  TUPLE_PASSES_PRELIMINARY_CHECKS,
  TUPLE_FAILS_TO_RECOVER_AUTHORITY_ADDRESS,
  TUPLE_FAILS_DUE_TO_AUTHORITY_NEITHER_HAVING_EMPTY_CODE_NOR_BEING_DELEGATED,
  TUPLE_FAILS_DUE_TO_NONCE_MISMATCH,
  TUPLE_IS_VALID,
  ;

  public boolean passesPreliminaryChecks() {
    return !failsPreliminaryChecks();
  }

  public boolean failsPreliminaryChecks() {
    return this == TUPLE_FAILS_CHAIN_ID_CHECK
        || this == TUPLE_FAILS_NONCE_RANGE_CHECK
        || this == TUPLE_FAILS_S_RANGE_CHECK;
  }

  public boolean isInvalid() {
    return this != TUPLE_IS_VALID;
  }
}
