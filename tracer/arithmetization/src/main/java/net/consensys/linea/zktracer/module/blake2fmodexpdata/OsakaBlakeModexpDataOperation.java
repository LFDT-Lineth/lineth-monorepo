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
package net.consensys.linea.zktracer.module.blake2fmodexpdata;

import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.TraceOsaka.Blake2fmodexpdata.*;

import net.consensys.linea.zktracer.module.hub.precompiles.modexpMetadata.OsakaModexpMetadata;

public class OsakaBlakeModexpDataOperation extends LondonBlakeModexpDataOperation {

  public OsakaBlakeModexpDataOperation(OsakaModexpMetadata modexpMetaData, int id) {
    super(modexpMetaData, id);
  }

  public static short modexpComponentByteSize() {
    return LLARGE * (INDEX_MAX_MODEXP + 1);
  }

  @Override
  public short getIndexMaxModexpBase() {
    return INDEX_MAX_MODEXP_BASE;
  }

  @Override
  public short getIndexMaxModexpExponent() {
    return INDEX_MAX_MODEXP_EXPONENT;
  }

  @Override
  public short getIndexMaxModexpModulus() {
    return INDEX_MAX_MODEXP_MODULUS;
  }

  @Override
  public short getIndexMaxModexpResult() {
    return INDEX_MAX_MODEXP_RESULT;
  }
}
