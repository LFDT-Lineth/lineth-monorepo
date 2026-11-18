# @lfdt-lineth/native-libs

`@lfdt-lineth/native-libs` is a Node.js library that provides an interface to native Go libraries using the `koffi` package.
It provides the following Go libraries wrapper:

- `GoNativeCompressor`: This class allows you to initialize the transaction compressor, check for errors, and get the worst compressed transaction size for a given RLP-encoded transaction.

## Installation

Install the required npm package:

```bash
npm install @lfdt-lineth/native-libs
```

## Building

The build downloads the native binaries from a `blob-libs` GitHub release and bundles them in `dist/lib`:

```bash
NATIVE_LIBS_RELEASE_TAG=blob-libs-v3.0.1 pnpm run -F @lfdt-lineth/native-libs build
```

| Variable                  | Required | Description                                                                                                                                                                |
| ------------------------- | -------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `NATIVE_LIBS_RELEASE_TAG` | Yes      | Release to download the binaries from, for example `blob-libs-v3.0.1`.                                                                                                     |
| `NATIVE_LIBS_TARGETS`     | No       | Binaries to keep: `all`, or a comma-separated list of `linux-x64`, `linux-arm64`, `darwin-x64`, `darwin-arm64`. Defaults to the platform of the machine running the build. |

Each build replaces the binaries from previous builds. To bundle binaries for another platform, for example when preparing files for a Linux machine from a Mac:

```bash
NATIVE_LIBS_RELEASE_TAG=blob-libs-v3.0.1 NATIVE_LIBS_TARGETS=linux-x64,linux-arm64 pnpm run -F @lfdt-lineth/native-libs build
```

## Usage

### Compressor library

#### Importing the Class

```javascript
import { GoNativeCompressor } from '@lfdt-lineth/native-libs';
```

#### Initializing the Compressor

Create an instance of `GoNativeCompressor` by providing a data size limit:

```javascript
const dataLimit = 1024; // Example data limit
const compressor = new GoNativeCompressor(dataLimit);
```

#### Getting the Compressed Transaction Size

To get the worst compressed transaction size for a given RLP-encoded transaction:

```javascript
const rlpEncodedTransaction = new Uint8Array([...]); // Your RLP-encoded transaction
const compressedTxSize = compressor.getCompressedTxSize(rlpEncodedTransaction);
console.log(`Compressed Transaction Size: ${compressedTxSize}`);
```

#### Methods

#### `constructor(dataLimit: number)`

- **Parameters:**
  - `dataLimit`: The data limit for the compressor.

- **Description:** Initializes the compressor with the given data limit and loads the native library.

#### `getCompressedTxSize(rlpEncodedTransaction: Uint8Array): number`

- **Parameters:**
  - `rlpEncodedTransaction`: The RLP-encoded transaction as a `Uint8Array`.

- **Returns:** The worst compressed transaction size as a `number`.

- **Description:** Returns the worst compressed transaction size for the given RLP-encoded transaction. Throws an error if compression fails.

#### `getError(): string | null`

- **Returns:** The error message as a `string` or `null` if no error.

- **Description:** Retrieves the last error message from the native library.

#### Error Handling

If an error occurs during initialization or compression, an `Error` will be thrown with a descriptive message.

#### Example

```javascript
import { GoNativeCompressor } from '@lfdt-lineth/native-libs';

const dataLimit = 1024;
const compressor = new GoNativeCompressor(dataLimit);

const rlpEncodedTransaction = new Uint8Array([...]);
try {
  const compressedTxSize = compressor.getCompressedTxSize(rlpEncodedTransaction);
  console.log(`Compressed Transaction Size: ${compressedTxSize}`);
} catch (error) {
  console.error(`Error: ${error.message}`);
}
```

## License

This project is licensed under the Apache-2.0 and the MIT licenses.
