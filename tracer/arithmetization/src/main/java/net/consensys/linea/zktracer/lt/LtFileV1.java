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
package net.consensys.linea.zktracer.lt;

import java.io.IOException;
import java.io.OutputStream;
import java.nio.ByteBuffer;
import java.util.List;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.module.Module;

public class LtFileV1 extends LtFile {
  private final Trace.ColumnHeader[] columnHeaders;
  private final Column.Raw[] columns;

  public LtFileV1(Header header, Trace trace, List<Module> modules) {
    super(header);
    // Determine set of all columns headers
    final List<Trace.ColumnHeader> headers =
        modules.stream().flatMap(m -> m.columnHeaders(trace).stream()).toList();
    //
    this.columnHeaders = alignHeaders(headers);
    this.columns = new Column.Raw[columnHeaders.length];

    for (int i = 0; i < columnHeaders.length; i++) {
      Trace.ColumnHeader colHeader = columnHeaders[i];
      if (colHeader != null) {
        columns[i] = new Column.Raw(colHeader.name(), colHeader.bitwidth(), colHeader.length());
      }
    }
  }

  @Override
  protected Trace.Column[] columns() {
    return columns;
  }

  /**
   * Write a binary representation of the trace to a given output stream.
   *
   * @param out
   * @return
   */
  @Override
  public void write(OutputStream out) throws IOException {
    // Write header for LTv1 file
    out.write(header.toBytes());
    out.write(getColumnHeaderBytes(columnHeaders));
    // Write column data
    for (int i = 0; i != columns.length; i++) {
      if (columns[i] != null) {
        out.write(columns[i].toBytes());
      }
    }
    // Done
    out.flush();
  }

  /**
   * Align headers ensures that the order in which columns are seen matches the order found in the
   * trace schema.
   *
   * @param headers The headers to be aligned.
   * @return The aligned headers.
   */
  private static Trace.ColumnHeader[] alignHeaders(List<Trace.ColumnHeader> headers) {
    int maxRegister = 0;
    // Determine largest register
    //
    for (Trace.ColumnHeader header : headers) {
      maxRegister = Math.max(header.register(), maxRegister);
    }
    //
    Trace.ColumnHeader[] alignedHeaders = new Trace.ColumnHeader[maxRegister + 1];
    //
    for (Trace.ColumnHeader header : headers) {
      alignedHeaders[header.register()] = header;
    }
    //
    return alignedHeaders;
  }

  /**
   * Write header information for the trace file.
   *
   * @param headers Column headers.*
   */
  private static byte[] getColumnHeaderBytes(Trace.ColumnHeader[] headers) throws IOException {
    ByteBuffer buffer = ByteBuffer.allocate(getColumnHeadersSize(headers));
    // Write column count as uint32
    buffer.putInt(countHeaders(headers));
    // Write column headers one-by-one
    for (Trace.ColumnHeader h : headers) {
      if (h != null) {
        byte[] bytes = h.name().getBytes();
        buffer.putShort((short) bytes.length);
        buffer.put(bytes);
        buffer.put((byte) Util.byteWidth(h.bitwidth()));
        buffer.putInt(h.length());
      }
    }
    //
    return buffer.array();
  }

  /**
   * Precompute the size of the trace file in order to memory map the buffers.
   *
   * @param headers Set of headers for the columns being written.
   * @return Number of bytes requires for the trace file header.
   */
  private static int getColumnHeadersSize(Trace.ColumnHeader[] headers) {
    int nBytes = 4; // column count

    for (Trace.ColumnHeader header : headers) {
      if (header != null) {
        byte[] bytes = header.name().getBytes();
        nBytes += 2; // name length
        nBytes += bytes.length;
        nBytes += 1; // byte per element
        nBytes += 4; // element count
      }
    }

    return nBytes;
  }

  /**
   * Counter number of active (i.e. non-null) headers. A header can be null if it represents a
   * column in a module which is not activated for this trace.
   */
  private static int countHeaders(Trace.ColumnHeader[] headers) {
    int count = 0;
    for (Trace.ColumnHeader h : headers) {
      if (h != null) {
        count++;
      }
    }
    return count;
  }
}
