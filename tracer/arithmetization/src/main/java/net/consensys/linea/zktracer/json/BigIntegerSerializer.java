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

package net.consensys.linea.zktracer.json;

import com.fasterxml.jackson.core.JsonGenerator;
import com.fasterxml.jackson.databind.SerializerProvider;
import com.fasterxml.jackson.databind.ser.std.StdSerializer;
import java.io.IOException;
import java.math.BigInteger;

/** Custom JSON serializer for {@link BigInteger} type. */
public class BigIntegerSerializer extends StdSerializer<BigInteger> {
  private static final BigInteger INTEGER_MAX = BigInteger.valueOf(Integer.MAX_VALUE);

  public BigIntegerSerializer() {
    this(null);
  }

  public BigIntegerSerializer(final Class<BigInteger> t) {
    super(t);
  }

  @Override
  public void serialize(
      final BigInteger value, final JsonGenerator gen, final SerializerProvider provider)
      throws IOException {
    if (value.compareTo(INTEGER_MAX) > 0) {
      gen.writeString(value.toString());
    } else {
      gen.writeNumber(value.intValue());
    }
  }
}
