/*
 * lineth_da: the conflation compressor and the DA stream of the RISC-V rollup guest, as a C library.
 *
 * Ownership
 *   - The caller owns every input. The library copies what it keeps, the dictionary of a
 *     compressor and the shared blob a stream opens inside, and holds no pointer of the caller
 *     once a call returns.
 *   - The library owns what it returns, a lineth_da_compressed or a lineth_da_sealed_blobs
 *     including the segment ends of each blob, until the matching *_free call. A *_free call
 *     accepts NULL and a result that holds nothing, and clears the result.
 *   - A handle lives until lineth_da_compressor_free or lineth_da_stream_close. Both accept NULL.
 *
 * Errors
 *   Every call that can fail returns a status. A failure leaves its `out` cleared and writes a
 *   NUL-terminated message into `err`, cut to `err_cap` bytes; `err` may be NULL. A NULL pointer
 *   that the call needs, `out` included, is LINETH_DA_ERR_INVALID_ARGUMENT.
 *
 * Threads
 *   lineth_da_compress may run from any number of threads at once, on one compressor or on
 *   several. A stream is used by one thread at a time. Streams are independent of each other.
 *
 * Versions
 *   LINETH_DA_ABI_VERSION is bumped on any change to the structs, the enum or the signatures, and
 *   lineth_da_abi_version returns the one the library was built with. Statuses are appended, never
 *   reordered. The library produces the DA format of the rollup guest built from the same commit,
 *   so a release pairs with a rollup guest release.
 */
#ifndef LINETH_DA_H
#define LINETH_DA_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define LINETH_DA_ABI_VERSION     1
#define LINETH_DA_BLOB_BYTES      131072
#define LINETH_DA_KZG_POINT_BYTES 48

/* Statuses are appended, never reordered: callers map them by value. */
typedef enum {
  LINETH_DA_OK = 0,
  LINETH_DA_ERR_INVALID_ARGUMENT,
  LINETH_DA_ERR_INVALID_DICTIONARY,
  LINETH_DA_ERR_INVALID_BLOCK_RLP,
  LINETH_DA_ERR_SEEDED_BLOB_MISMATCH,
  LINETH_DA_ERR_OUT_OF_MEMORY,
  LINETH_DA_ERR_INTERNAL,
} lineth_da_status;

typedef struct lineth_da_compressor lineth_da_compressor;
typedef struct lineth_da_stream     lineth_da_stream;

typedef struct {
  const uint8_t *ptr;
  size_t len;
} lineth_da_bytes;

/* The conflation of a compress call as one zstd frame. */
typedef struct {
  uint8_t *frame;
  size_t frame_len;
} lineth_da_compressed;

/* The segment of the conflation ending at conflation_end_block ends in a blob, followed by
 * unused_tail_bytes of payload. */
typedef struct {
  uint64_t conflation_end_block;
  uint32_t unused_tail_bytes;
} lineth_da_segment_end;

/* A packed blob with its KZG commitment and proof, its versioned hash (chunk_hash) and the data
 * rolling hash before and after it. segment_ends holds segment_end_count entries, NULL for none. */
typedef struct {
  uint8_t  blob[LINETH_DA_BLOB_BYTES];
  uint8_t  chunk_hash[32];
  uint8_t  kzg_commitment[LINETH_DA_KZG_POINT_BYTES];
  uint8_t  kzg_proof[LINETH_DA_KZG_POINT_BYTES];
  uint8_t  prev_data_rolling_hash[32];
  uint8_t  data_rolling_hash[32];
  uint32_t payload_bytes;
  uint32_t segment_end_count;
  const lineth_da_segment_end *segment_ends;
} lineth_da_sealed_blob;

/* The blobs of a stream call, in order: `count` entries, NULL for none. */
typedef struct {
  lineth_da_sealed_blob *blobs;
  size_t count;
} lineth_da_sealed_blobs;

/* The end of the last proven rollup. With a zero shared_blob_carry_over_tail_bytes the stream
 * starts on a fresh blob after data_rolling_hash, and parent_blob_data_rolling_hash and
 * shared_blob are ignored. Otherwise the rollup ended inside shared_blob, a blob of
 * LINETH_DA_BLOB_BYTES whose last shared_blob_carry_over_tail_bytes of payload carry over to the
 * next rollup, and parent_blob_data_rolling_hash is the data rolling hash before that blob. */
typedef struct {
  uint8_t  data_rolling_hash[32];
  uint8_t  parent_blob_data_rolling_hash[32];
  uint32_t shared_blob_carry_over_tail_bytes;
  const uint8_t *shared_blob;
} lineth_da_stream_position;

/* The LINETH_DA_ABI_VERSION the library was built with. */
uint32_t lineth_da_abi_version(void);

/* Creates a compressor over a zstd dictionary that declares an ID, for blocks of chain_id.
 * INVALID_DICTIONARY when the bytes are not such a dictionary. */
lineth_da_status lineth_da_compressor_new(const uint8_t *dictionary, size_t dictionary_len, uint64_t chain_id,
                                          lineth_da_compressor **out, char *err, size_t err_cap);
void             lineth_da_compressor_free(lineth_da_compressor *compressor);

/* Truncates the full block RLPs of one conflation, in order, and compresses them into one frame.
 * The caller checks the block numbers, the links between blocks and the chain ID of the
 * transactions, and gives the frame the last block of the conflation when it appends it to a
 * stream. INVALID_ARGUMENT without blocks, INVALID_BLOCK_RLP for a block that cannot be decoded or
 * whose sender cannot be recovered. */
lineth_da_status lineth_da_compress(const lineth_da_compressor *compressor, const lineth_da_bytes *block_rlps,
                                    size_t block_count, lineth_da_compressed *out, char *err, size_t err_cap);
void             lineth_da_compressed_free(lineth_da_compressed *compressed);

/* Opens a stream at a position. INVALID_ARGUMENT when the shared blob is not the canonical packing
 * of a payload longer than the tail, or does not fold from parent_blob_data_rolling_hash to
 * data_rolling_hash. */
lineth_da_status lineth_da_stream_open(const lineth_da_stream_position *position, lineth_da_stream **out,
                                       char *err, size_t err_cap);
void             lineth_da_stream_close(lineth_da_stream *stream);

/* Appends the segment of a conflation whose last block is conflation_end_block, in order and once
 * each, and returns the blobs it sealed. INVALID_ARGUMENT for an empty frame or one too long for
 * the four-byte length prefix. SEEDED_BLOB_MISMATCH when the bytes differ from the shared blob the
 * stream opened inside. A failed call leaves the stream as it was. */
lineth_da_status lineth_da_stream_append(lineth_da_stream *stream, const uint8_t *frame, size_t frame_len,
                                         uint64_t conflation_end_block, lineth_da_sealed_blobs *out,
                                         char *err, size_t err_cap);

/* Seals the open blob at the end of the stream: no blob when none is open. INVALID_ARGUMENT while
 * the stream has bytes of its shared blob left to reproduce. */
lineth_da_status lineth_da_stream_flush(lineth_da_stream *stream, lineth_da_sealed_blobs *out,
                                        char *err, size_t err_cap);
void             lineth_da_sealed_blobs_free(lineth_da_sealed_blobs *sealed);

#ifdef __cplusplus
}
#endif

#endif /* LINETH_DA_H */
