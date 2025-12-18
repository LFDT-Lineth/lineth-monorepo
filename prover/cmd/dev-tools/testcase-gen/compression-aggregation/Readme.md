Implements a simple module capable of producing sample output from the blob compression, for testing purpose on the contract. The tool takes the form of a CLI tool.

## Building and using

```sh
cd prover
make bin/compression-aggregation-sample
bin/compression-aggregation-sample --help
```

Which will output the following usage:

```
Usage:
  generate [flags]

Flags:
  -h, --help               help for generate
      --odir string        Output directory where to write the sample files (default ".")
      --spec stringArray   JSON spec file to use to generate a sequence of blobs
```

## Spec files

Specs files are settings that can be passed to the data generator to control how the random blobs are structured. Example:

### For compression

```javascript
{
    "blobSubmissionSpec": {
      // If this flag is set, it tells the generator to use the data that is specified in the current spec file rather than the data obtained from the previous blob submissions. This is handy when generating "invalid" sample files      
      "ignoreBefore": true,

      "startFromL2Block": 1, // First L2 block in the sequence
      "numConflation": 5, // First conflation in the sequence
      "blockPerConflation": 12, // Number of L2 blocks in the sequence (this is in fact a maxima)
      "compressedDataSize": 110000, // Sized of the compressed data. The compressed data is not obtained by actually compressing data, it is just random
      "parentShnarf": "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", // Shnarf to build on top of
      "parentStateRootHash": "0x0000000000000000000000000000000000000000000000000000000000000000" // Prev state-root hash. Only there to be passed to the contract.
    }
}
```

### For aggregation

```javascript
{
    "aggregationSpec": {

      // If this flag is set, it tells the generator to use the data that is specified in the current spec file rather than the data obtained from the previous blob submissions. This is handy when generating "invalid" sample files
      "ignoreBefore": false,
      "dataParentHash": "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "lastFinalizedTimestamp": 10000000,
      "finalTimestamp": 10001000,
      "l1RollingHash": "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
      "l1RollingHashMessageNumber": 12,
      "howManyL2Msgs": 30,
      "treeDepth": 5,
      "l2MessagingBlocksOffsets": "0x0000000000000000000000000fffff"
    }
}

```

## Several blobs in a row

You can pass several time the spec flags. This will tell the CLI tool to generate blobs in a row. Example:

```sh
bin/compression-aggregation-sample --odir ./compression-sample --spec ./cmd/dev-tools/testcase-gen/compression/spec-comp.json --spec ./cmd/dev-tools/testcase-gen/compression/spec-comp.json
```

## Several blobs in a row then an aggregation

You can finalize a serie of blob submissions by an aggregation with the following command:

**1 submission and 1 finalization**

```sh
bin/compression-aggregation-sample --odir .samples-test/ --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-agg.json
```

**4 blob submission then 1 aggregation**

```sh
bin/compression-aggregation-sample --odir .samples-simple/ --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-agg.json
```

**2 blob submission, 1 aggregation, 2 blob submission, 1 aggregation**

```sh
bin/compression-aggregation-sample --odir .samples-multiproof/ --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-agg.json

bin/compression-aggregation-sample --odir .samples-multiproof/ --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-2.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-2.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-agg-2.json --seed 1
```

**Update the contracts tests

```sh
SOLTESTDIR=../contracts/test/testData/compressedData
rm ${SOLTESTDIR}/blocks* ${SOLTESTDIR}/aggregatedProof*
rm ${SOLTESTDIR}/multipleProofs/blocks* ${SOLTESTDIR}/multipleProofs/aggregatedProof*
mv .samples-simple/* ${SOLTESTDIR} 
mv .samples-multiproof/* ${SOLTESTDIR}/multipleProofs 
mv .samples-testing/* ${SOLTESTDIR}/test
```