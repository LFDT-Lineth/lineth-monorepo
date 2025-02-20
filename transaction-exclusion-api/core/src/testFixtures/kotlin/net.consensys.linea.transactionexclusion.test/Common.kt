package net.consensys.linea.transactionexclusion.test

import kotlinx.datetime.Instant
import linea.kotlin.decodeHex
import net.consensys.linea.transactionexclusion.ModuleOverflow
import net.consensys.linea.transactionexclusion.RejectedTransaction
import net.consensys.linea.transactionexclusion.TransactionInfo

val defaultRejectedTransaction = RejectedTransaction(
  txRejectionStage = RejectedTransaction.Stage.SEQUENCER,
  timestamp = Instant.parse("2024-08-31T09:18:51Z"),
  blockNumber = 10000UL,
  transactionRLP =
  (
    "0x02f8388204d2648203e88203e88203e8941195cf65f83b3a5768f3c4" +
      "96d3a05ad6412c64b38203e88c666d93e9cc5f73748162cea9c0017b8201c8"
    )
    .decodeHex(),
  reasonMessage = "Transaction line count for module ADD=402 is above the limit 70",
  overflows = listOf(
    ModuleOverflow(
      module = "ADD",
      count = 402,
      limit = 70
    ),
    ModuleOverflow(
      module = "MUL",
      count = 587,
      limit = 401
    ),
    ModuleOverflow(
      module = "EXP",
      count = 9000,
      limit = 8192
    )
  ),
  transactionInfo = TransactionInfo(
    hash = "0x526e56101cf39c1e717cef9cedf6fdddb42684711abda35bae51136dbb350ad7".decodeHex(),
    from = "0x4d144d7b9c96b26361d6ac74dd1d8267edca4fc2".decodeHex(),
    to = "0x1195cf65f83b3a5768f3c496d3a05ad6412c64b3".decodeHex(),
    nonce = 100UL
  )
)

val rejectedContractDeploymentTransaction = RejectedTransaction(
  txRejectionStage = RejectedTransaction.Stage.RPC,
  timestamp = Instant.parse("2024-10-31T09:18:51Z"),
  blockNumber = null,
  transactionRLP = (
    "0xb8d602f8d382e708018403780fc08403780fca830118fd8080b87960566023600b82828239805160001" +
      "a607314601657fe5b30600052607381538281f3fe73000000000000000000000000000000000000000030" +
      "146080604052600080fdfea2646970667358221220c399228be3777f235999f42c0d0a666dc163e66f600" +
      "fdbfcc0a0500a4683db1064736f6c63430007060033c001a0a905ece7f3784afa2130063e332899fa60eb" +
      "13863d96cea29810808c7d5a18eea0685b5237be1e44ccf7d4a9da4410a48cab5a23ba51e23fe3598294c7d34108c1"
    ).decodeHex(),
  reasonMessage = "Transaction 0x583eb047887cc72f93ead08f389a2cd84440f3322bc4b191803d5adb0a167525 " +
    "line count for module HUB=2119318 is above the limit 2097152",
  overflows = listOf(
    ModuleOverflow(
      module = "HUB",
      count = 2119318,
      limit = 2097152
    )
  ),
  transactionInfo = TransactionInfo(
    hash = "0x583eb047887cc72f93ead08f389a2cd84440f3322bc4b191803d5adb0a167525".decodeHex(),
    from = "0x0d06838d1dfba9ef0a4166cca9be16fb1d76dbfc".decodeHex(),
    to = null,
    nonce = 1UL
  )
)
