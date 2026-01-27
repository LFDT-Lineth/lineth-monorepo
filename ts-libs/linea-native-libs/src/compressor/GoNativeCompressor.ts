import { KoffiFunction, load } from "koffi";
import path from "path";

import { getCompressorLibPath } from "./helpers";

const COMPRESSOR_DICT_PATH = path.join(__dirname, "./lib/25-04-21.bin");

/**
 * Class representing a Go Native Compressor.
 */
export class GoNativeCompressor {
  private initFunc: KoffiFunction;
  private errorFunc: KoffiFunction;
  private worstCompressedTxSizeFunc: KoffiFunction;
  /**
   * Creates an instance of GoNativeCompressor.
   * @param {number} dataLimit - The data limit for the compressor.
   * @throws {Error} Throws an error if initialization fails.
   */
  constructor(dataLimit: number) {
    const compressorLibPath = getCompressorLibPath();
    const lib = load(compressorLibPath);
    this.initFunc = lib.func("Init", "bool", ["int", "char*"]);
    this.errorFunc = lib.func("Error", "char*", []);
    this.worstCompressedTxSizeFunc = lib.func("WorstCompressedTxSize", "int", ["char*", "int"]);

    this.init(dataLimit);
  }

  /**
   * Initializes the compressor with the given data limit.
   * @param {number} dataLimit - The data limit for the compressor.
   * @returns {boolean} Returns `true` if initialization is successful.
   * @throws {Error} Throws an error if initialization fails.
   * @private
   */
  private init(dataLimit: number): boolean {
    const isInitSuccess = this.initFunc(dataLimit, COMPRESSOR_DICT_PATH);
    if (!isInitSuccess) {
      const error = this.getError();
      throw new Error(`Error while initialization the compressor library. error=${error}`);
    }
    return isInitSuccess;
  }

  /**
   * Gets the worst compressed transaction size for a given RLP-encoded transaction.
   * @param {Uint8Array} rlpEncodedTransaction - The RLP-encoded transaction.
   * @returns {number} The worst compressed transaction size.
   * @throws {Error} Throws an error if compression fails.
   */
  public getCompressedTxSize(rlpEncodedTransaction: Uint8Array): number {
    const compressedTxSize = this.worstCompressedTxSizeFunc(rlpEncodedTransaction, rlpEncodedTransaction.byteLength);

    const error = this.getError();
    if (error) {
      throw new Error(`Error while compressing the transaction: ${error}`);
    }
    return compressedTxSize;
  }

  /**
   * Retrieves the last error message from the native library.
   * @returns {string | null} The error message or null if no error.
   * @private
   */
  private getError(): string | null {
    try {
      return this.errorFunc();
    } catch {
      return null;
    }
  }
}
