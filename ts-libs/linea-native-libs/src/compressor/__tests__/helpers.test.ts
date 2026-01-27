import { describe, it, expect } from "@jest/globals";
import fs from "fs";
import os from "os";
import path from "path";

import { getCompressorLibPath } from "../helpers";

describe("Helpers", () => {
  afterEach(() => {
    jest.clearAllMocks();
  });

  describe("getCompressorLibPath", () => {
    it("Should throw an error when the os platform is not supported", () => {
      jest.mock("os");
      const platform = "android";
      jest.spyOn(os, "platform").mockReturnValueOnce(platform);

      expect(() => getCompressorLibPath()).toThrow(`Unsupported platform: ${platform}`);
    });

    it("Should throw an error when the resources folder does not exist", () => {
      jest.spyOn(fs, "existsSync").mockReturnValueOnce(false);

      const platform = os.platform();
      const arch = os.arch();
      const dirPath = path.resolve("src", "compressor", "lib", `${platform}-${arch}`);

      expect(() => getCompressorLibPath()).toThrow(`Directory does not exist: ${dirPath}`);
    });

    it("Should throw an error when the lib file does not exist", () => {
      jest.spyOn(fs, "existsSync").mockReturnValueOnce(true);
      jest.spyOn(fs, "readdirSync").mockReturnValueOnce([]);

      const platform = os.platform();
      const arch = os.arch();
      const dirPath = path.resolve("src", "compressor", "lib", `${platform}-${arch}`);

      expect(() => getCompressorLibPath()).toThrow(`No matching library file found in directory: ${dirPath}`);
    });

    it("Should return lib compressor", async () => {
      jest.mock("os");
      jest.spyOn(fs, "existsSync").mockReturnValueOnce(true);
      const filename = "blob_compressor_v0.1.0.dylib";
      (jest.spyOn(fs, "readdirSync") as jest.Mock).mockReturnValueOnce([filename]);
      const platform = "darwin";
      const arch = "arm64";
      jest.spyOn(os, "platform").mockReturnValueOnce(platform);
      jest.spyOn(os, "arch").mockReturnValueOnce("arm64");

      const dirPath = path.resolve("src", "compressor", "lib", `${platform}-${arch}`);
      const libPath = getCompressorLibPath();

      expect(libPath).toStrictEqual(`${dirPath}/${filename}`);
    });
  });
});
