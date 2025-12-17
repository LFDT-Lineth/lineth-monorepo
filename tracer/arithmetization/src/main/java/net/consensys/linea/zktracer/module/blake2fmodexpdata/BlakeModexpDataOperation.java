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

package net.consensys.linea.zktracer.module.blake2fmodexpdata;

import static net.consensys.linea.zktracer.Trace.Blake2fmodexpdata.*;
import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.Trace.PHASE_BLAKE_DATA;
import static net.consensys.linea.zktracer.Trace.PHASE_BLAKE_PARAMS;
import static net.consensys.linea.zktracer.Trace.PHASE_BLAKE_RESULT;
import static net.consensys.linea.zktracer.Trace.PHASE_MODEXP_BASE;
import static net.consensys.linea.zktracer.Trace.PHASE_MODEXP_EXPONENT;
import static net.consensys.linea.zktracer.Trace.PHASE_MODEXP_MODULUS;
import static net.consensys.linea.zktracer.Trace.PHASE_MODEXP_RESULT;
import static net.consensys.linea.zktracer.TraceOsaka.Blake2fmodexpdata.INDEX_MAX_MODEXP_BASE;
import static net.consensys.linea.zktracer.TraceOsaka.Blake2fmodexpdata.INDEX_MAX_MODEXP_EXPONENT;
import static net.consensys.linea.zktracer.TraceOsaka.Blake2fmodexpdata.INDEX_MAX_MODEXP_MODULUS;
import static net.consensys.linea.zktracer.TraceOsaka.Blake2fmodexpdata.INDEX_MAX_MODEXP_RESULT;
import static net.consensys.linea.zktracer.types.Utils.leftPadTo;

import java.util.Optional;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata;
import net.consensys.linea.zktracer.types.UnsignedByte;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.Hash;

@Accessors(fluent = true)
public final class BlakeModexpDataOperation extends ModuleOperation {
  public static final short BLAKE2f_R_SIZE = 4;
  public static final short BLAKE2f_HASH_INPUT_OFFSET = BLAKE2f_R_SIZE;
  public static final short BLAKE2f_HASH_INPUT_SIZE = LLARGE * (INDEX_MAX_BLAKE_DATA + 1);
  public static final short BLAKE2f_HASH_OUTPUT_SIZE = LLARGE * (INDEX_MAX_BLAKE_RESULT + 1);

  public static short numberOfRowsBlake() {
    return (INDEX_MAX_BLAKE_DATA + 1) + (INDEX_MAX_BLAKE_PARAMS + 1) + (INDEX_MAX_BLAKE_RESULT + 1);
  }

  private short getIndexMaxModexpBase() {
    return INDEX_MAX_MODEXP_BASE;
  }

  private short getIndexMaxModexpExponent() {
    return INDEX_MAX_MODEXP_EXPONENT;
  }

  private short getIndexMaxModexpModulus() {
    return INDEX_MAX_MODEXP_MODULUS;
  }

  short getIndexMaxModexpResult() {
    return INDEX_MAX_MODEXP_RESULT;
  }

  public short numberOfRowsModexp() {
    return (short)
        ((getIndexMaxModexpBase() + 1)
            + (getIndexMaxModexpExponent() + 1)
            + (getIndexMaxModexpModulus() + 1)
            + (getIndexMaxModexpResult() + 1));
  }

  @Getter public final long id;

  public final Optional<ModexpMetadata> modexpMetaData;
  public final Optional<BlakeComponents> blake2fComponents;

  public BlakeModexpDataOperation(final ModexpMetadata modexpMetaData, final int id) {
    this.id = id;
    this.modexpMetaData = Optional.of(modexpMetaData);
    this.blake2fComponents = Optional.empty();
  }

  public BlakeModexpDataOperation(final BlakeComponents blakeComponents, final int id) {
    this.id = id;
    this.modexpMetaData = Optional.empty();
    this.blake2fComponents = Optional.of(blakeComponents);
  }

  @Override
  protected int computeLineCount() {
    return modexpMetaData.isPresent() ? numberOfRowsModexp() : numberOfRowsBlake();
  }

  void trace(Trace.Blake2fmodexpdata trace, final int stamp) {

    if (modexpMetaData.isPresent()) {
      traceModexpBase(trace, stamp);
      traceModexpExponent(trace, stamp);
      traceModexpModulus(trace, stamp);
      traceModexpResult(trace, stamp);
      return;
    }

    if (blake2fComponents.isPresent()) {
      traceBlakeData(trace, stamp);
      traceBlakeParameter(trace, stamp);
      traceBlakeResult(trace, stamp);
    }
  }

  private void traceBlakeData(Trace.Blake2fmodexpdata trace, int stamp) {
    final Bytes input = blake2fComponents.get().getHashInput();
    for (int index = 0; index <= INDEX_MAX_BLAKE_DATA; index++) {
      commonTrace(trace, stamp, index, input, INDEX_MAX_BLAKE_DATA);
      trace.phase(UnsignedByte.of(PHASE_BLAKE_DATA)).isBlakeData(true).fillAndValidateRow();
    }
  }

  private void traceBlakeParameter(Trace.Blake2fmodexpdata trace, int stamp) {
    // r
    commonTrace(
        trace, stamp, 0, leftPadTo(blake2fComponents.get().r(), LLARGE), INDEX_MAX_BLAKE_PARAMS);
    trace.phase(UnsignedByte.of(PHASE_BLAKE_PARAMS)).isBlakeParams(true).fillAndValidateRow();

    // f
    commonTrace(
        trace,
        stamp,
        1,
        leftPadTo(blake2fComponents.get().f(), 2 * LLARGE),
        INDEX_MAX_BLAKE_PARAMS);
    trace.phase(UnsignedByte.of(PHASE_BLAKE_PARAMS)).isBlakeParams(true).fillAndValidateRow();
  }

  private void traceBlakeResult(Trace.Blake2fmodexpdata trace, int stamp) {
    final Bytes hash = computeBlake2fResult();
    for (int index = 0; index <= INDEX_MAX_BLAKE_RESULT; index++) {
      commonTrace(trace, stamp, index, hash, INDEX_MAX_BLAKE_RESULT);
      trace.phase(UnsignedByte.of(PHASE_BLAKE_RESULT)).isBlakeResult(true).fillAndValidateRow();
    }
  }

  private void traceModexpBase(Trace.Blake2fmodexpdata trace, final int stamp) {
    final Bytes input =
        leftPadTo(modexpMetaData.get().base(), modexpMetaData.get().getMaxInputSize());
    for (int index = 0; index <= getIndexMaxModexpBase(); index++) {
      commonTrace(trace, stamp, index, input, getIndexMaxModexpBase());
      trace.phase(UnsignedByte.of(PHASE_MODEXP_BASE)).isModexpBase(true).fillAndValidateRow();
    }
  }

  private void traceModexpExponent(Trace.Blake2fmodexpdata trace, final int stamp) {
    final Bytes input =
        leftPadTo(modexpMetaData.get().exp(), modexpMetaData.get().getMaxInputSize());
    for (int index = 0; index <= getIndexMaxModexpExponent(); index++) {
      commonTrace(trace, stamp, index, input, getIndexMaxModexpExponent());
      trace
          .phase(UnsignedByte.of(PHASE_MODEXP_EXPONENT))
          .isModexpExponent(true)
          .fillAndValidateRow();
    }
  }

  private void traceModexpModulus(Trace.Blake2fmodexpdata trace, final int stamp) {
    final Bytes input =
        leftPadTo(modexpMetaData.get().mod(), modexpMetaData.get().getMaxInputSize());
    for (int index = 0; index <= getIndexMaxModexpModulus(); index++) {
      commonTrace(trace, stamp, index, input, getIndexMaxModexpModulus());
      trace.phase(UnsignedByte.of(PHASE_MODEXP_MODULUS)).isModexpModulus(true).fillAndValidateRow();
    }
  }

  private void traceModexpResult(Trace.Blake2fmodexpdata trace, final int stamp) {
    final Bytes input =
        leftPadTo(modexpMetaData.get().rawResult(), modexpMetaData.get().getMaxInputSize());
    for (int index = 0; index <= getIndexMaxModexpResult(); index++) {
      commonTrace(trace, stamp, index, input, getIndexMaxModexpResult());
      trace.phase(UnsignedByte.of(PHASE_MODEXP_RESULT)).isModexpResult(true).fillAndValidateRow();
    }
  }

  private void commonTrace(
      Trace.Blake2fmodexpdata trace, int stamp, int index, Bytes input, int indexMax) {
    trace
        .stamp(stamp)
        .id(id)
        .index(UnsignedByte.of(index))
        .indexMax(UnsignedByte.of(indexMax))
        .limb(input.slice(index * LLARGE, LLARGE));
  }

  private Bytes computeBlake2fResult() {
    return Hash.blake2bf(blake2fComponents.get().callData());
  }

  public boolean isModexpOperation() {
    return modexpMetaData.isPresent();
  }

  public boolean isBlakeOperation() {
    return blake2fComponents.isPresent();
  }
}
