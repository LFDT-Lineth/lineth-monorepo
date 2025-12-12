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

package net.consensys.linea.zktracer.types;

final class BytesValues {

  static byte[] fromRawHexString(CharSequence str, int destSize, boolean lenient) {
    int len = str.length();
    CharSequence hex = str;
    if (len >= 2 && str.charAt(0) == '0' && str.charAt(1) == 'x') {
      hex = str.subSequence(2, len);
      len -= 2;
    }

    int idxShift = 0;
    if ((len & 0x01) != 0) {
      if (!lenient) {
        throw new IllegalArgumentException("Invalid odd-length hex binary representation");
      }

      hex = "0" + hex;
      len += 1;
      idxShift = 1;
    }

    int size = len >> 1;
    if (destSize < 0) {
      destSize = size;
    } else {
      Checks.checkArgument(
          size <= destSize,
          "Hex value is too large: expected at most %s bytes but got %s",
          destSize,
          size);
    }

    byte[] out = new byte[destSize];

    int destOffset = (destSize - size);
    for (int i = destOffset, j = 0; j < len; i++) {
      int h = Character.digit(hex.charAt(j), 16);
      if (h == -1) {
        throw new IllegalArgumentException(
            String.format(
                "Illegal character '%c' found at index %d in hex binary representation",
                hex.charAt(j), j - idxShift));
      }
      j++;
      int l = Character.digit(hex.charAt(j), 16);
      if (l == -1) {
        throw new IllegalArgumentException(
            String.format(
                "Illegal character '%c' found at index %d in hex binary representation",
                hex.charAt(j), j - idxShift));
      }
      j++;
      out[i] = (byte) ((h << 4) + l);
    }
    return out;
  }
}
