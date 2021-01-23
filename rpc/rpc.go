package rpc

import (
	"fmt"
	"math/big"

	"github.com/harmony-one/go-sdk/pkg/common"
	"github.com/harmony-one/harmony-tf/config"
	"github.com/harmony-one/harmony-tf/logger"
	"github.com/harmony-one/harmony-tf/testing"
	"github.com/harmony-one/harmony-tf/transactions"

	sdkAccounts "github.com/harmony-one/go-lib/accounts"
	sdkTxs "github.com/harmony-one/go-lib/transactions"
)

// These aren't exposed by harmony-one/harmony since they are internal... need to copy them
var (
	EthMainnetChainID            = big.NewInt(1666600000)
	EthTestnetChainID            = big.NewInt(1666700000)
	EthPangaeaChainID            = big.NewInt(1666800000)
	EthPartnerChainID            = big.NewInt(1666900000)
	EthStressnetChainID          = big.NewInt(1661000000)
	EthTestChainID               = big.NewInt(1661100000) // not a real network
	EthAllProtocolChangesChainID = big.NewInt(1661200000) // not a real network
)

// SendTransaction - sends a transaction and switches RPC settings if necessary
func SendTransaction(testCase *testing.TestCase, senderAccount *sdkAccounts.Account, receiverAccount *sdkAccounts.Account) sdkTxs.Transaction {
	if testCase.Parameters.RPCPrefix == "eth" {
		ethChainID := GenerateEthereumChainID(config.Configuration.Network.Name, testCase.Parameters.FromShardID)
		config.Configuration.Network.ChangeRPCSettings(testCase.Parameters.RPCPrefix, ethChainID)
	}

	testCaseTx := SendGenericTransaction(testCase, senderAccount, receiverAccount)

	config.Configuration.Network.RevertRPCSettings()

	return testCaseTx
}

// SendGenericTransaction - sends a regular tx or eth tx using the supplied function
func SendGenericTransaction(testCase *testing.TestCase, senderAccount *sdkAccounts.Account, receiverAccount *sdkAccounts.Account) sdkTxs.Transaction {
	var rawTx map[string]interface{}
	var err error

	txData := testCase.Parameters.GenerateTxData()

	logger.TransactionLog(fmt.Sprintf("Sending transaction of %f token(s) from %s (shard %d) to %s (shard %d), tx data size: %d byte(s)", testCase.Parameters.Amount, senderAccount.Address, testCase.Parameters.FromShardID, receiverAccount.Address, testCase.Parameters.ToShardID, len(txData)), testCase.Verbose)
	logger.TransactionLog(fmt.Sprintf("Will wait up to %d seconds to let the transaction get finalized", testCase.Parameters.Timeout), testCase.Verbose)

	if testCase.Parameters.RPCPrefix == "eth" {
		rawTx, err = transactions.SendEthTransaction(senderAccount, testCase.Parameters.FromShardID, receiverAccount.Address, testCase.Parameters.Amount, testCase.Parameters.Nonce, testCase.Parameters.Gas.Limit, testCase.Parameters.Gas.Price, txData, testCase.Parameters.Timeout)
	} else {
		rawTx, err = transactions.SendTransaction(senderAccount, testCase.Parameters.FromShardID, receiverAccount.Address, testCase.Parameters.ToShardID, testCase.Parameters.Amount, testCase.Parameters.Nonce, testCase.Parameters.Gas.Limit, testCase.Parameters.Gas.Price, txData, testCase.Parameters.Timeout)
	}

	testCaseTx := sdkTxs.ToTransaction(senderAccount.Address, testCase.Parameters.FromShardID, receiverAccount.Address, testCase.Parameters.ToShardID, rawTx, err)

	return testCaseTx
}

// GenerateEthereumChainID - map a network name and a shard ID to the corresponding Ethereum version
func GenerateEthereumChainID(networkName string, shardID uint32) *common.ChainID {
	switch networkName {
	case "mainnet":
		return &common.ChainID{Name: "eth_mainnet", Value: EthereumChainIDForShard(EthMainnetChainID, shardID)}
	case "testnet", "localnet":
		return &common.ChainID{Name: "eth_testnet", Value: EthereumChainIDForShard(EthTestnetChainID, shardID)}
	case "pangaea":
		return &common.ChainID{Name: "eth_pangaea", Value: EthereumChainIDForShard(EthPangaeaChainID, shardID)}
	case "devnet", "partner":
		return &common.ChainID{Name: "eth_partnernet", Value: EthereumChainIDForShard(EthPartnerChainID, shardID)}
	case "stressnet":
		return &common.ChainID{Name: "eth_stressnet", Value: EthereumChainIDForShard(EthStressnetChainID, shardID)}
	case "dryrun":
		return &common.ChainID{Name: "eth_dryrun", Value: EthereumChainIDForShard(EthMainnetChainID, shardID)}
	default:
		return &common.ChainID{Name: "eth_mainnet", Value: EthereumChainIDForShard(EthMainnetChainID, shardID)}
	}
}

// EthereumChainIDForShard - calculate an Ethereum chainID based on a given shard ID
func EthereumChainIDForShard(baseShardID *big.Int, shardID uint32) *big.Int {
	return big.NewInt(0).Add(baseShardID, big.NewInt(int64(shardID)))
}
