import { writeFileSync } from "fs";

/**
 * Escape a single CSV field, wrapping it in double quotes when it contains a
 * comma, double quote, carriage return or line feed.
 * @param value The value to escape.
 * @returns The CSV-safe string representation of the value.
 */
function escapeCsvValue(value: unknown): string {
  if (value === null || value === undefined) {
    return "";
  }

  const stringValue = typeof value === "object" ? JSON.stringify(value) : String(value);

  if (/[",\r\n]/.test(stringValue)) {
    return `"${stringValue.replace(/"/g, '""')}"`;
  }

  return stringValue;
}

/**
 * Serialize headers and rows into a CSV string.
 * @param headers Ordered list of column names.
 * @param rows Rows keyed by column name.
 * @returns The CSV content as a string (with a trailing newline).
 */
export function toCsv(headers: string[], rows: Record<string, unknown>[]): string {
  const headerLine = headers.map(escapeCsvValue).join(",");
  const dataLines = rows.map((row) => headers.map((header) => escapeCsvValue(row[header])).join(","));
  return [headerLine, ...dataLines].join("\n") + "\n";
}

/**
 * Write headers and rows to a CSV file.
 * @param filePath Destination file path.
 * @param headers Ordered list of column names.
 * @param rows Rows keyed by column name.
 */
export function writeCsvFile(filePath: string, headers: string[], rows: Record<string, unknown>[]): void {
  writeFileSync(filePath, toCsv(headers, rows), "utf8");
}
