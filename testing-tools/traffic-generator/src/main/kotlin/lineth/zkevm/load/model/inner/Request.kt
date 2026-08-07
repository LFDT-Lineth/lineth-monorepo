package lineth.zkevm.load.model.inner

import lineth.zkevm.load.model.EthConnection
import lineth.zkevm.load.swagger.TransferOwnership
import java.math.BigInteger

const val NEW = "new"
const val SOURCE_WALLET = "source"

class Request(val id: Int, val name: String, val calls: List<ScenarioDefinition>, val context: Context) {

  companion object {
    fun translate(fromJson: lineth.zkevm.load.swagger.Request?): Request {
      if (fromJson == null) {
        return Request(-1, "null", listOf(), Context(-1, listOf(), "url", 1))
      }
      return Request(
        fromJson.id!!,
        fromJson.name ?: "",
        fromJson.calls!!.map { c -> translate(c) }.toList(),
        translate(fromJson.context),
      )
    }

    fun translate(fromJson: lineth.zkevm.load.swagger.ScenarioDefinition): ScenarioDefinition {
      return ScenarioDefinition(fromJson.nbOfExecution ?: 1, translate(fromJson.scenario!!))
    }

    private fun translate(transaction: lineth.zkevm.load.swagger.Scenario): Scenario {
      return when (transaction.scenarioType) {
        "RoundRobinMoneyTransfer" -> {
          val transfer = transaction as lineth.zkevm.load.swagger.RoundRobinMoneyTransfer
          RoundRobinMoneyTransfer(NEW, transfer.nbTransfers ?: 1, transfer.nbWallets ?: 1)
        }

        "SelfTransactionWithPayload" -> {
          val transfer = transaction as lineth.zkevm.load.swagger.SelfTransactionWithPayload
          SelfTransactionWithPayload(
            transfer.wallet ?: NEW,
            transaction.nbWallets ?: 1,
            transaction.nbTransfers ?: 1,
            transaction.payload ?: "",
            if (transaction.price == null) {
              EthConnection.SIMPLE_TX_PRICE
            } else {
              BigInteger.valueOf(transaction.price!!.toLong())
            },
          )
        }

        "SelfTransactionWithRandomPayload" -> {
          val transfer = transaction as lineth.zkevm.load.swagger.SelfTransactionWithRandomPayload
          SelfTransactionWithRandomPayload(
            transfer.wallet ?: NEW,
            transaction.nbWallets ?: 1,
            transaction.nbTransfers ?: 1,
            transaction.payloadSize ?: 0,
            if (transaction.price == null) {
              EthConnection.SIMPLE_TX_PRICE
            } else {
              BigInteger.valueOf(transaction.price!!.toLong())
            },
          )
        }

        "ContractCall" -> {
          val contractCall = transaction as lineth.zkevm.load.swagger.ContractCall
          ContractCall(transaction.wallet ?: SOURCE_WALLET, translate(contractCall.contract!!))
        }

        else -> {
          throw UnsupportedOperationException(transaction.toJson())
        }
      }
    }

    private fun translate(contract: lineth.zkevm.load.swagger.Contract): Contract {
      return when (contract.contractCallType) {
        "CallExistingContract" -> {
          contract as lineth.zkevm.load.swagger.CallExistingContract
          CallExistingContract(contract.contractAddress!!, translate(contract.getMethodAndParameters()!!))
        }

        "CreateContract" -> {
          contract as lineth.zkevm.load.swagger.CreateContract
          createContract(contract)
        }

        "CallContractReference" -> {
          contract as lineth.zkevm.load.swagger.CallContractReference
          CallContractReference(contract.contractName!!, translate(contract.getMethodAndParameters()!!))
        }

        else -> {
          throw UnsupportedOperationException(contract.toJson())
        }
      }
    }

    private fun createContract(contract: lineth.zkevm.load.swagger.CreateContract): CreateContract {
      val gasLimit =
        contract.gasLimit
          ?: throw IllegalArgumentException(
            "CreateContract `${contract.name ?: "null"}` is missing required gasLimit.",
          )
      return CreateContract(contract.name ?: "null", contract.byteCode, gasLimit)
    }

    private fun translate(methodAndParams: lineth.zkevm.load.swagger.MethodAndParameter): MethodAndParameter {
      return when (methodAndParams.type) {
        "GenericCall" -> {
          methodAndParams as lineth.zkevm.load.swagger.GenericCall
          GenericCall(
            methodAndParams.numberOfTimes,
            methodAndParams.methodName!!,
            methodAndParams.price!!,
            methodAndParams.parameters?.map { p -> translate(p) }?.toList()!!,
          )
        }

        "Mint" -> {
          methodAndParams as lineth.zkevm.load.swagger.Mint
          Mint(
            methodAndParams.numberOfTimes,
            methodAndParams.address ?: "self",
            methodAndParams.amount
              ?: 0,
          )
        }

        "BatchMint" -> {
          methodAndParams as lineth.zkevm.load.swagger.BatchMint
          BatchMint(
            methodAndParams.numberOfTimes,
            methodAndParams.address
              ?: listOf("self"),
            methodAndParams.amount ?: 0,
          )
        }

        "TransferOwnership" -> {
          methodAndParams as TransferOwnership
          TransferOwnerShip(
            methodAndParams.numberOfTimes,
            methodAndParams.destinationAddress
              ?: "self",
          )
        }

        else -> {
          throw UnsupportedOperationException(methodAndParams.toJson())
        }
      }
    }

    private fun translate(parameter: lineth.zkevm.load.swagger.Parameter): Parameter {
      return when (parameter.type) {
        "ArrayParameter" -> {
          parameter as lineth.zkevm.load.swagger.ArrayParameter
          ArrayParameter(parameter.values?.map { p -> translate(p) }!!)
        }

        "SimpleParameter" -> {
          parameter as lineth.zkevm.load.swagger.SimpleParameter
          SimpleParameter(parameter.value!!, parameter.solidityType!!)
        }

        else -> {
          throw UnsupportedOperationException(parameter.toJson())
        }
      }
    }

    private fun translate(context: lineth.zkevm.load.swagger.Context?): Context {
      return Context(
        context?.chainId!!,
        context.contracts?.map { c -> createContract(c) }?.toList() ?: listOf(),
        context.url!!,
        context.nbOfExecutions!!,
      )
    }
  }
}
