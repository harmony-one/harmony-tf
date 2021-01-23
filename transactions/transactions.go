package transactions

import (
	"encoding/base64"

	sdkAccounts "github.com/harmony-one/go-lib/accounts"
	sdkNetworkNonce "github.com/harmony-one/go-lib/network/rpc/nonces"
	sdkTxs "github.com/harmony-one/go-lib/transactions"
	"github.com/harmony-one/go-sdk/pkg/rpc"
	"github.com/harmony-one/harmony-tf/config"
	"github.com/harmony-one/harmony/numeric"
)

// SendTransaction - send transactions
func SendTransaction(account *sdkAccounts.Account, fromShardID uint32, toAddress string, toShardID uint32, amount numeric.Dec, nonce int, gasLimit int64, gasPrice numeric.Dec, txData string, timeout int) (map[string]interface{}, error) {
	account.Unlock()

	rpcClient, currentNonce, err := TransactionPrerequisites(account, fromShardID, nonce)
	if err != nil {
		return nil, err
	}

	if len(txData) > 0 {
		txData = base64.StdEncoding.EncodeToString([]byte(txData))
	}

	txResult, err := sdkTxs.SendTransaction(account.Keystore, account.Account, rpcClient, config.Configuration.Network.API.ChainID, account.Address, fromShardID, toAddress, toShardID, amount, gasLimit, gasPrice, currentNonce, txData, config.Configuration.Account.Passphrase, config.Configuration.Network.API.NodeAddress(fromShardID), timeout)

	if err != nil {
		return nil, err
	}

	return txResult, nil
}

// SendSameShardTransaction - send a transaction using the same shard for both the receiver and the sender
func SendSameShardTransaction(account *sdkAccounts.Account, toAddress string, shardID uint32, amount numeric.Dec, nonce int, gasLimit int64, gasPrice numeric.Dec, txData string, timeout int) (map[string]interface{}, error) {
	return SendTransaction(account, shardID, toAddress, shardID, amount, nonce, gasLimit, gasPrice, txData, timeout)
}

// SendEthTransaction - send transactions
func SendEthTransaction(account *sdkAccounts.Account, shardID uint32, toAddress string, amount numeric.Dec, nonce int, gasLimit int64, gasPrice numeric.Dec, txData string, timeout int) (map[string]interface{}, error) {
	account.Unlock()

	rpcClient, currentNonce, err := TransactionPrerequisites(account, shardID, nonce)
	if err != nil {
		return nil, err
	}

	if len(txData) > 0 {
		txData = base64.StdEncoding.EncodeToString([]byte(txData))
	}

	nodeAddress := config.Configuration.Network.API.NodeAddress(shardID)

	txResult, err := sdkTxs.SendEthTransaction(account.Keystore, account.Account, rpcClient, config.Configuration.Network.API.ChainID, account.Address, toAddress, amount, gasLimit, gasPrice, currentNonce, txData, config.Configuration.Account.Passphrase, nodeAddress, timeout)

	if err != nil {
		return nil, err
	}

	return txResult, nil
}

// TransactionPrerequisites - resolves required clients to perform transactions
func TransactionPrerequisites(account *sdkAccounts.Account, shardID uint32, nonce int) (*rpc.HTTPMessenger, uint64, error) {
	rpcClient, err := config.Configuration.Network.API.RPCClient(shardID)
	if err != nil {
		return nil, 0, err
	}

	var currentNonce uint64
	if nonce < 0 {
		currentNonce = sdkNetworkNonce.CurrentNonce(rpcClient, account.Address)
		if err != nil {
			return nil, 0, err
		}
	} else {
		currentNonce = uint64(nonce)
	}

	return rpcClient, currentNonce, nil
}
