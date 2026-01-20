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

package net.consensys.linea.zktracer.module.blake2f;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.module.blake2fmodexpdata.BlakeComponents;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.apache.tuweni.units.bigints.UInt256;
import org.hyperledger.besu.crypto.Hash;

import java.util.HexFormat;
import java.util.Optional;

import static net.consensys.linea.zktracer.types.Conversions.bytesToBoolean;
import static net.consensys.linea.zktracer.types.Conversions.longToBytes;

@Accessors(fluent = true)
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class Blake2fOperation extends ModuleOperation {
  public static final short BLAKE2f_H_INPUT_CHUNK_SIZE = 16;
  public static final short BLAKE2f_M_CHUNK_SIZE = 16;
  static final short BLAKE2f_T_SIZE = 16;

  @EqualsAndHashCode.Include public final BlakeComponents blake2fComponents;

  public Blake2fOperation(BlakeComponents blakeComponents) {
      this.blake2fComponents = blakeComponents;
  }

  public void trace(Trace.Blake2f trace) {
    Bytes result = Hash.blake2bf(blake2fComponents.callData());
    trace.r(blake2fComponents.r())
      .h0h1BeInput(blake2fComponents.getHashInput().slice(0, BLAKE2f_H_INPUT_CHUNK_SIZE))
      .h2h3BeInput(blake2fComponents.getHashInput().slice(16, BLAKE2f_H_INPUT_CHUNK_SIZE))
      .h4h5BeInput(blake2fComponents.getHashInput().slice(32, BLAKE2f_H_INPUT_CHUNK_SIZE))
      .h6h7BeInput(blake2fComponents.getHashInput().slice(48, BLAKE2f_H_INPUT_CHUNK_SIZE))
      .m0m1Be(blake2fComponents.getHashInput().slice(64, BLAKE2f_M_CHUNK_SIZE))
      .m2m3Be(blake2fComponents.getHashInput().slice(80, BLAKE2f_M_CHUNK_SIZE))
      .m4m5Be(blake2fComponents.getHashInput().slice(96, BLAKE2f_M_CHUNK_SIZE))
      .m6m7Be(blake2fComponents.getHashInput().slice(112, BLAKE2f_M_CHUNK_SIZE))
      .m8m9Be(blake2fComponents.getHashInput().slice(128, BLAKE2f_M_CHUNK_SIZE))
      .m10m11Be(blake2fComponents.getHashInput().slice(144, BLAKE2f_M_CHUNK_SIZE))
      .m12m13Be(blake2fComponents.getHashInput().slice(160, BLAKE2f_M_CHUNK_SIZE))
      .m14m15Be(blake2fComponents.getHashInput().slice(176, BLAKE2f_M_CHUNK_SIZE))
      .t0t1Be(blake2fComponents.getHashInput().slice(192, BLAKE2f_T_SIZE))
      .f(bytesToBoolean(blake2fComponents.f()))
      .h0h1Be(result.slice(0, BLAKE2f_H_INPUT_CHUNK_SIZE))
      .h2h3Be(result.slice(16, BLAKE2f_H_INPUT_CHUNK_SIZE))
      .h4h5Be(result.slice(32, BLAKE2f_H_INPUT_CHUNK_SIZE))
      .h6h7Be(result.slice(48, BLAKE2f_H_INPUT_CHUNK_SIZE))
      .fillAndValidateRow();
 }

  @Override
  protected int computeLineCount() {
    return 1;
  }
}
