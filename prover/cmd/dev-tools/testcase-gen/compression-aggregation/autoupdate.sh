#!/bin/sh

make bin/compression-aggregation-sample

# This is commented out because the autoupdate.sh does not do anything with this
# bin/compression-aggregation-sample --odir ./compression-sample-calldata --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-calldata-1.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-calldata-1.json
bin/compression-aggregation-sample --odir .samples-test-calldata/ --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-calldata-1.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-agg.json
bin/compression-aggregation-sample --odir .samples-simple-calldata/ --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-calldata-1.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-calldata-1.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-calldata-1.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-calldata-1.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-agg.json
bin/compression-aggregation-sample --odir .samples-multiproof-calldata/ --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-calldata-1.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-calldata-1.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-agg.json
bin/compression-aggregation-sample --odir .samples-multiproof-calldata/ --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-calldata-2.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-calldata-2.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-agg-2.json

# This is commented out because the autoupdate.sh does not do anything with this
# bin/compression-aggregation-sample --odir ./compression-sample-eip4844 --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-eip4844-1.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-eip4844-1.json
bin/compression-aggregation-sample --odir .samples-test-eip4844/ --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-eip4844-1.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-agg.json
bin/compression-aggregation-sample --odir .samples-simple-eip4844/ --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-eip4844-1.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-eip4844-1.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-eip4844-1.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-eip4844-1.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-agg.json
bin/compression-aggregation-sample --odir .samples-multiproof-eip4844/ --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-eip4844-1.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-eip4844-1.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-agg.json
bin/compression-aggregation-sample --odir .samples-multiproof-eip4844/ --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-eip4844-2.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-comp-eip4844-2.json --spec ./cmd/dev-tools/testcase-gen/compression-aggregation/spec-agg-2.json

SOLTESTDIR=../contracts/test/hardhat/_testData/compressedData
SOLTESTDIR_EIP4844=../contracts/test/hardhat/_testData/compressedDataEip4844

mkdir -p ${SOLTESTDIR} ${SOLTESTDIR}/multipleProofs  ${SOLTESTDIR}/test
mkdir -p ${SOLTESTDIR_EIP4844} ${SOLTESTDIR_EIP4844}/multipleProofs  ${SOLTESTDIR_EIP4844}/test

rm -f ${SOLTESTDIR}/blocks* ${SOLTESTDIR}/aggregatedProof*
rm -f ${SOLTESTDIR}/multipleProofs/blocks* ${SOLTESTDIR}/multipleProofs/aggregatedProof*
mv -f .samples-simple-calldata/* ${SOLTESTDIR} 
mv -f .samples-multiproof-calldata/* ${SOLTESTDIR}/multipleProofs 
mv -f .samples-test-calldata/* ${SOLTESTDIR}/test

rm -f ${SOLTESTDIR_EIP4844}/blocks* ${SOLTESTDIR_EIP4844}/aggregatedProof*
rm -f ${SOLTESTDIR_EIP4844}/multipleProofs/blocks* ${SOLTESTDIR_EIP4844}/multipleProofs/aggregatedProof*
mv -f .samples-simple-eip4844/* ${SOLTESTDIR_EIP4844} 
mv -f .samples-multiproof-eip4844/* ${SOLTESTDIR_EIP4844}/multipleProofs 
mv -f .samples-test-eip4844/* ${SOLTESTDIR_EIP4844}/test

rm -rf .samples-simple-calldata .samples-multiproof-calldata .samples-test-calldata
rm -rf .samples-simple-eip4844 .samples-multiproof-eip4844 .samples-test-eip4844

sed -i.bak 's/pragma solidity \0.8.24;/pragma solidity 0.8.26;/g' ../contracts/test/hardhat/_testData/compressedData/Verifier1.sol

cp ../contracts/test/hardhat/_testData/compressedData/Verifier1.sol ../contracts/src/verifiers/PlonkVerifierForDataAggregation.sol
sed -i.bak 's/contract PlonkVerifier /contract PlonkVerifierForDataAggregation /g' ../contracts/src/verifiers/PlonkVerifierForDataAggregation.sol

cp ../contracts/test/hardhat/_testData/compressedData/Verifier1.sol ../contracts/src/_testing/unit/verifiers/TestPlonkVerifierForDataAggregation.sol
sed -i.bak 's/contract PlonkVerifier /contract TestPlonkVerifierForDataAggregation /g' ../contracts/src/_testing/unit/verifiers/TestPlonkVerifierForDataAggregation.sol

rm  ../contracts/src/_testing/unit/verifiers/TestPlonkVerifierForDataAggregation.sol.bak
rm  ../contracts/src/verifiers/PlonkVerifierForDataAggregation.sol.bak

# Remove this artefact from the code. This litters the contracts tests
rm ../contracts/test/hardhat/_testData/**/Verifier1.*
rm ../contracts/test/hardhat/_testData/**/**/Verifier1.*