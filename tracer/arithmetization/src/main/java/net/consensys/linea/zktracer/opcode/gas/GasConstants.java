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

package net.consensys.linea.zktracer.opcode.gas;

import static net.consensys.linea.zktracer.Trace.*;

import lombok.RequiredArgsConstructor;

/** All the classes of gas prices per instruction used in the EVM. */
@RequiredArgsConstructor
public enum GasConstants {
  G_ZERO(GAS_CONST_G_ZERO),
  G_JUMP_DEST(GAS_CONST_G_JUMPDEST),
  G_BASE(GAS_CONST_G_BASE),
  G_VERY_LOW(GAS_CONST_G_VERY_LOW),
  G_LOW(GAS_CONST_G_LOW),
  G_MID(GAS_CONST_G_MID),
  G_HIGH(GAS_CONST_G_HIGH),
  G_WARM_ACCESS(GAS_CONST_G_WARM_ACCESS),
  G_ACCESS_LIST_ADDRESS(GAS_CONST_G_ACCESS_LIST_ADRESS),
  G_ACCESS_LIST_STORAGE(GAS_CONST_G_ACCESS_LIST_STORAGE),
  G_PER_EMPTY_ACCOUNT_COST(GAS_CONST_G_PER_EMPTY_ACCOUNT_COST),
  G_COLD_ACCOUNT_ACCESS(GAS_CONST_G_COLD_ACCOUNT_ACCESS),
  G_COLD_S_LOAD(GAS_CONST_G_COLD_SLOAD),
  G_S_SET(GAS_CONST_G_SSET),
  G_S_RESET(GAS_CONST_G_SRESET),
  R_S_CLEAR(REFUND_CONST_R_SCLEAR),
  G_SELF_DESTRUCT(GAS_CONST_G_SELFDESTRUCT),
  G_CREATE(GAS_CONST_G_CREATE),
  G_CODE_DEPOSIT(GAS_CONST_G_CODE_DEPOSIT),
  G_CALL_VALUE(GAS_CONST_G_CALL_VALUE),
  G_CALL_STIPEND(GAS_CONST_G_CALL_STIPEND),
  G_NEW_ACCOUNT(GAS_CONST_G_NEW_ACCOUNT),
  G_EXP(GAS_CONST_G_EXP),
  G_EXP_BYTE(GAS_CONST_G_EXP_BYTE),
  G_MEMORY(GAS_CONST_G_MEMORY),
  G_TX_CREATE(GAS_CONST_G_TX_CREATE),
  G_TX_DATA_ZERO(GAS_CONST_G_TX_DATA_ZERO),
  G_TX_DATA_NON_ZERO(GAS_CONST_G_TX_DATA_NONZERO),
  G_TRANSACTION(GAS_CONST_G_TRANSACTION),
  G_LOG_0(GAS_CONST_G_LOG),
  G_LOG_1(GAS_CONST_G_LOG + GAS_CONST_G_LOG_TOPIC),
  G_LOG_2(GAS_CONST_G_LOG + 2 * GAS_CONST_G_LOG_TOPIC),
  G_LOG_3(GAS_CONST_G_LOG + 3 * GAS_CONST_G_LOG_TOPIC),
  G_LOG_4(GAS_CONST_G_LOG + 4 * GAS_CONST_G_LOG_TOPIC),
  G_LOG_DATA(GAS_CONST_G_LOG_DATA),
  G_LOG_TOPIC(GAS_CONST_G_LOG_TOPIC),
  G_KECCAK_256(GAS_CONST_G_KECCAK_256),
  G_KECCAK_256_WORD(GAS_CONST_G_KECCAK_256_WORD),
  G_COPY(GAS_CONST_G_COPY),
  G_BLOCK_HASH(GAS_CONST_G_BLOCKHASH),
  // below are markers for gas that is computed in other modules
  // that is: hub, memory expansion, stipend, precompile info
  S_MXP(0),
  S_CALL(0),
  // computing the cost of a CALL requires HUB data (warmth, account existence, ...), MXP
  // data for memory expansion, STP data for gas stipend <- made it its own type
  S_HUB(0),
  S_STP(0),
  S_PREC_INFO(0),
  // EIP-3860: CREAE(2) are added GAS_CONST_INIT_CODE_WORD = 2 per word for the init code
  G_CONST_INIT_CODE_WORD(GAS_CONST_INIT_CODE_WORD),
  G_CONST_INIT_CODE_WORD_CREATE2(GAS_CONST_G_KECCAK_256_WORD + GAS_CONST_INIT_CODE_WORD),
  G_CONST_HASH_OPCODE(GAS_CONST_HASH_OPCODE_GAS);

  /** The gas price of the instruction family. */
  private final int cost;

  public int cost() {
    return this.cost;
  }
}
