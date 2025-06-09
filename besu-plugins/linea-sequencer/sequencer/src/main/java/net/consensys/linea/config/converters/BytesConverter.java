/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.config.converters;

import org.apache.tuweni.bytes.Bytes;
import picocli.CommandLine;

public class BytesConverter implements CommandLine.ITypeConverter<Bytes> {
  @Override
  public Bytes convert(final String s) throws Exception {
    return Bytes.fromHexStringLenient(s);
  }
}
