{
  "config": {
    "chainId": 1337,
    "petersburgBlock": 0,
    "istanbulBlock": 0,
    "berlinBlock": 0,
    "londonBlock": 0,
    "terminalTotalDifficulty":0,
    "cancunTime":0,
    "pragueTime":0,
    "blobSchedule": {
      "cancun": {
        "target": 0,
        "max": 0,
        "baseFeeUpdateFraction": 3338477
      },
      "prague": {
        "target": 0,
        "max": 1,
        "baseFeeUpdateFraction": 5007716
      },
      "osaka": {
        "target": 0,
        "max": 0,
        "baseFeeUpdateFraction": 5007716
      }
    },
    "clique": {
      "blockperiodseconds": %blockperiodseconds%,
      "epochlength": %epochlength%,
      "createemptyblocks": %createemptyblocks%
    },
    "depositContractAddress": "0x4242424242424242424242424242424242424242",
    "withdrawalRequestContractAddress": "0x00A3ca265EBcb825B45F985A16CEFB49958cE017",
    "consolidationRequestContractAddress": "0x00b42dbF2194e931E80326D950320f7d9Dbeac02"
  },
  "zeroBaseFee": false,
  "baseFeePerGas": "7",
  "nonce": "0x0",
  "timestamp": "0x0",
  "extraData": "%extraData%",
  "gasLimit": "0x1C9C380",
  "difficulty": "0x1",
  "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
  "coinbase": "0x0000000000000000000000000000000000000000",
  "alloc": {
    "fe3b557e8fb62b89f4916b721be55ceb828dbd73": {
      "privateKey": "8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63",
      "comment": "private key and this comment are ignored.  In a real chain, the private key should NOT be stored",
      "balance": "0xad78ebc5ac6200000"
    },
    "627306090abaB3A6e1400e9345bC60c78a8BEf57": {
      "privateKey": "c87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3",
      "comment": "private key and this comment are ignored.  In a real chain, the private key should NOT be stored",
      "balance": "90000000000000000000000"
    },
    "f17f52151EbEF6C7334FAD080c5704D77216b732": {
      "privateKey": "ae6ae8e5ccbfb04590405997ee2d52d2b330726137b875053c36d94e974d162f",
      "comment": "private key and this comment are ignored.  In a real chain, the private key should NOT be stored",
      "balance": "90000000000000000000000"
    },
    "a05b21E5186Ce93d2a226722b85D6e550Ac7D6E3": {
      "privateKey": "3a4ff6d22d7502ef2452368165422861c01a0f72f851793b372b87888dc3c453",
      "balance": "90000000000000000000000"
    },
    "8da48afC965480220a3dB9244771bd3afcB5d895": {
      "comment": "This account has signed a authorization for contract 0x0000000000000000000000000000000000009999 to send a 7702 transaction",
      "privateKey": "11f2e7b6a734ab03fa682450e0d4681d18a944f8b83c99bf7b9b4de6c0f35ea1",
      "balance": "90000000000000000000000"
    },
    "0x0000000000000000000000000000000000000666": {
      "comment": "Contract reverts immediately when called",
      "balance": "0",
      "code": "5F5FFD",
      "codeDecompiled": "PUSH0 PUSH0 REVERT",
      "storage": {}
    },
    "0x0000000000000000000000000000000000009999": {
      "comment": "Contract sends all its Ether to the address provided as a call data.",
      "balance": "0",
      "code": "5F5F5F5F475F355AF100",
      "codeDecompiled": "PUSH0 PUSH0 PUSH0 PUSH0 SELFBALANCE PUSH0 CALLDATALOAD GAS CALL STOP",
      "storage": {}
    },
    "0xa4664C40AACeBD82A2Db79f0ea36C06Bc6A19Adb": {
      "balance": "1000000000000000000000000000"
    },
    "0x23618e81E3f5cdF7f54C3d65f7FBc0aBf5B21E8f": {
      "comment": "This is the account used to sign the transaction that creates a validator exit",
      "balance": "1000000000000000000000000000"
    },
    "0x00A3ca265EBcb825B45F985A16CEFB49958cE017": {
      "comment": "This is the runtime bytecode for the Withdrawal Request Smart Contract. It was created from the generated alloc section of fork_Prague_blockchain_test_engine_single_block_single_withdrawal_request_from_contract spec test",
      "balance": "0",
      "code": "0x3373fffffffffffffffffffffffffffffffffffffffe1460c7573615156028575f545f5260205ff35b36603814156101f05760115f54807fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff146101f057600182026001905f5b5f821115608057810190830284830290049160010191906065565b9093900434106101f057600154600101600155600354806003026004013381556001015f35815560010160203590553360601b5f5260385f601437604c5fa0600101600355005b6003546002548082038060101160db575060105b5f5b81811461017f5780604c02838201600302600401805490600101805490600101549160601b83528260140152807fffffffffffffffffffffffffffffffff0000000000000000000000000000000016826034015260401c906044018160381c81600701538160301c81600601538160281c81600501538160201c81600401538160181c81600301538160101c81600201538160081c81600101535360010160dd565b9101809214610191579060025561019c565b90505f6002555f6003555b5f54807fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff14156101c957505f5b6001546002828201116101de5750505f6101e4565b01600290035b5f555f600155604c025ff35b5f5ffd",
      "storage": {
        "0x0000000000000000000000000000000000000000000000000000000000000000": "0000000000000000000000000000000000000000000000000000000000000000",
        "0x0000000000000000000000000000000000000000000000000000000000000001": "0000000000000000000000000000000000000000000000000000000000000001",
        "0x0000000000000000000000000000000000000000000000000000000000000002": "0000000000000000000000000000000000000000000000000000000000000000",
        "0x0000000000000000000000000000000000000000000000000000000000000003": "0000000000000000000000000000000000000000000000000000000000000001",
        "0x0000000000000000000000000000000000000000000000000000000000000004": "000000000000000000000000a4664C40AACeBD82A2Db79f0ea36C06Bc6A19Adb",
        "0x0000000000000000000000000000000000000000000000000000000000000005": "b10a4a15bf67b328c9b101d09e5c6ee6672978fdad9ef0d9e2ceffaee9922355",
        "0x0000000000000000000000000000000000000000000000000000000000000006": "5d8601f0cb3bcc4ce1af9864779a416e00000000000000000000000000000000"
      }
    },
    "0x00b42dbF2194e931E80326D950320f7d9Dbeac02": {
      "comment": "This is the runtime bytecode for the Consolidation Request Smart Contract",
      "nonce": "0x01",
      "balance": "0x00",
      "code": "0x3373fffffffffffffffffffffffffffffffffffffffe14604d57602036146024575f5ffd5b5f35801560495762001fff810690815414603c575f5ffd5b62001fff01545f5260205ff35b5f5ffd5b62001fff42064281555f359062001fff015500",
      "storage": {}
    }
  },
  "number": "0x0",
  "gasUsed": "0x0",
  "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000"
}