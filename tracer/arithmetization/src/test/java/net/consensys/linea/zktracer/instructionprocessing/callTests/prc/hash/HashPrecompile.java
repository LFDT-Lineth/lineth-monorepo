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
package net.consensys.linea.zktracer.instructionprocessing.callTests.prc.hash;

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE;

import org.hyperledger.besu.datatypes.Address;

public enum HashPrecompile {
  SHA256,
  RIPEMD160,
  IDENTITY;

  public Address getAddress() {
    Address address =
        switch (this) {
          case SHA256 -> Address.SHA256;
          case RIPEMD160 -> Address.RIPEMD160;
          case IDENTITY -> Address.ID;
        };
    return address;
  }

  public HashPrecompile next() {
    return switch (this) {
      case SHA256 -> RIPEMD160;
      case RIPEMD160 -> IDENTITY;
      case IDENTITY -> SHA256;
    };
  }

  public int smallOffset1() {
    return switch (this) {
      case SHA256 -> 1;
      case RIPEMD160 -> 4;
      case IDENTITY -> 2;
    };
  }

  public int smallOffset2() {
    return switch (this) {
      case SHA256 -> 3;
      case RIPEMD160 -> 8;
      case IDENTITY -> 7;
    };
  }

  public int cost(int callDataSize) {
    int nWords = nWords(callDataSize);
    return switch (this) {
      case SHA256 -> 60 + 12 * nWords;
      case RIPEMD160 -> 600 + 120 * nWords;
      case IDENTITY -> 15 + 3 * nWords;
    };
  }

  private int nWords(int sizeInBytes) {
    checkArgument(sizeInBytes >= 0);
    if (sizeInBytes == 0) {
      return 0;
    } else {
      return (sizeInBytes + WORD_SIZE - 1) / WORD_SIZE;
    }
  }
}
