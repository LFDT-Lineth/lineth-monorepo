import { readFileSync } from "fs";

export function readFile(filePath: string): Buffer {
  try {
    return readFileSync(filePath);
  } catch (error) {
    throw new Error(
      `FileReadError: Unable to read file at ${filePath}. Error: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
}
