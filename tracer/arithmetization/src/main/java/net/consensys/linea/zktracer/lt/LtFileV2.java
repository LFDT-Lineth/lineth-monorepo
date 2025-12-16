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
import net.consensys.linea.zktracer.module.ModuleName;

public class LtFileV2 extends LtFile {
  private final ModuleHeader[] moduleHeaders;
  private final Column[] columns;
  private final BytesHeap heap;

  public LtFileV2(Header header, Trace trace, List<Module> modules) {
    super(header);
    this.moduleHeaders = new ModuleHeader[modules.size()];
    this.heap = new BytesHeap();
    int numColumnHeaders = 0;
    // Initialise module headers
    for (int i = 0; i < modules.size(); i++) {
      Module ith = modules.get(i);
      this.moduleHeaders[i] = new ModuleHeader(ith, trace, heap);
    }
    //
    this.columns = alignColumns(this.moduleHeaders);
  }

  @Override
  protected Trace.Column[] columns() {
    return columns;
  }

  @Override
  public void write(OutputStream out) throws IOException {
    final Encoding[][] encodings = getColumnEncodings(moduleHeaders);
    final byte[] heap = this.heap.toBytes();
    final byte[] headers = getModuleHeadersBytes(moduleHeaders, encodings);
    final ByteBuffer sizes = ByteBuffer.allocate(8);
    // Write header for LT file
    out.write(header.toBytes());
    // Write section sizes
    sizes.putInt(headers.length);
    sizes.putInt(heap.length);
    out.write(sizes.array());
    // Write section data
    out.write(headers);
    out.write(heap);
    // Write column data
    for (int i = 0; i != encodings.length; ++i) {
      ModuleHeader mod = moduleHeaders[i];
      for (int j = 0; j != encodings[i].length; ++j) {
        out.write(encodings[i][j].data());
      }
    }
    // Done
    out.flush();
  }

  /**
   * Determine all the column encodings.
   *
   * @param headers
   * @return
   * @throws IOException
   */
  private static Encoding[][] getColumnEncodings(ModuleHeader[] headers) throws IOException {
    Encoding[][] encodings = new Encoding[headers.length][];
    //
    for (int i = 0; i != headers.length; i++) {
      ModuleHeader ith = headers[i];
      Encoding[] ithEncodings = new Encoding[ith.columns.length];
      for (int j = 0; j != ith.columns.length; j++) {
        Column col = ith.columns[j];
        ithEncodings[j] = col.toEncoding();
      }
      encodings[i] = ithEncodings;
    }
    //
    return encodings;
  }

  //
  private static byte[] getModuleHeadersBytes(ModuleHeader[] headers, Encoding[][] encodings)
      throws IOException {
    byte[][] headerBytes = new byte[headers.length][];
    int length = 0;

    for (int i = 0; i != headers.length; ++i) {
      headerBytes[i] = headers[i].toBytes(encodings[i]);
      length += headerBytes[i].length;
    }
    // Flatten header bytes
    ByteBuffer buffer = ByteBuffer.allocate(length + 4);
    // Number of headers
    buffer.putInt(headers.length);
    //
    for (int i = 0; i != headerBytes.length; ++i) {
      buffer.put(headerBytes[i]);
    }
    //
    return buffer.array();
  }

  private static void writeName(String name, ByteBuffer buf) {
    byte[] bytes = name.getBytes();
    buf.putShort((short) bytes.length);
    buf.put(bytes);
  }

  private static class ModuleHeader {
    final String name;
    final long height;
    final List<Trace.ColumnHeader> headers;
    final Column[] columns;

    ModuleHeader(Module m, Trace trace, BytesHeap heap) {
      this.headers = m.columnHeaders(trace);
      name = extractModuleName(m.moduleKey(), headers);
      this.height = m.lineCount();
      this.columns = new Column[headers.size()];
      // initialise columns
      for (int i = 0; i != columns.length; i++) {
        Trace.ColumnHeader header = headers.get(i);
        this.columns[i] = Column.of(header, heap);
      }
    }

    int byteLength() {
      int length = 0;
      //
      length += 2; // name length
      length += name.getBytes().length; // name bytes
      length += 4; // column height
      length += 4; // number of columns
      //
      for (Column column : columns) {
        length += 2; // name length
        length += column.name().getBytes().length; // name bytes
        length += 4; // data length
        length += 4; // data encoding
        length += 2; // bitwidth
      }
      //
      return length;
    }

    private byte[] toBytes(Encoding[] encodings) throws IOException {
      ByteBuffer buf = ByteBuffer.allocate(byteLength());
      //
      writeName(name, buf);
      buf.putInt((int) height);
      buf.putInt(columns.length);
      //
      for (int i = 0; i != columns.length; i++) {
        Column col = columns[i];
        writeName(col.name(), buf);
        buf.putInt(encodings[i].data().length);
        buf.putInt(encodings[i].encoding());
        buf.putShort((short) col.bitwidth());
      }
      //
      return buf.array();
    }
  }

  /**
   * Alignment ensures that the order in which columns are seen matches the order found in the trace
   * schema.
   *
   * @param headers The headers to be aligned.
   * @return The aligned headers.
   */
  private static Column[] alignColumns(ModuleHeader[] headers) {
    int maxRegister = 0;
    // Determine largest register
    for (ModuleHeader m : headers) {
      for (Trace.ColumnHeader header : m.headers) {
        maxRegister = Math.max(header.register(), maxRegister);
      }
    }
    //
    Column[] alignedHeaders = new Column[maxRegister + 1];
    //
    for (int i = 0; i != headers.length; ++i) {
      Column[] columns = headers[i].columns;
      List<Trace.ColumnHeader> columnHeaders = headers[i].headers;
      for (int c = 0; c != columnHeaders.size(); c++) {
        Trace.ColumnHeader header = columnHeaders.get(c);
        alignedHeaders[header.register()] = columns[c];
      }
    }
    //
    return alignedHeaders;
  }

  /**
   * Attempt to extract the module name from the given list of column headers. This is really a
   * kludge to work around the fact that there is not enough information provided from the Trace
   * API. Eventually, this method should be removed.
   *
   * @param name
   * @param headers
   * @return
   */
  private static String extractModuleName(ModuleName name, List<Trace.ColumnHeader> headers) {
    if (headers.isEmpty()) {
      throw new IllegalArgumentException("module has no columns (" + name + ")");
    }
    String[] split = headers.getFirst().name().split("\\.");
    return split[0];
  }
}
