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

package net.consensys.linea.zktracer.opcode;

/** All the instruction families, as used by the hub. */
public enum InstructionFamily {
  ADD,
  MOD,
  MUL,
  EXT,
  WCP,
  BIN,
  SHF,
  KEC,
  CONTEXT,
  ACCOUNT,
  COPY,
  MCOPY,
  TRANSACTION,
  BATCH,
  STACK_RAM,
  STORAGE,
  TRANSIENT,
  JUMP,
  MACHINE_STATE,
  PUSH_POP,
  DUP,
  SWAP,
  LOG,
  CREATE,
  CALL,
  HALT,
  INVALID
}
