package parameters

import (
	sdkAccounts "github.com/harmony-one/go-lib/accounts"
	sdkNetworkTypes "github.com/harmony-one/go-lib/network/types/network"
	sdkTxs "github.com/harmony-one/go-lib/transactions"
	"github.com/harmony-one/go-sdk/pkg/common"
	"github.com/harmony-one/harmony-tf/config"
	"github.com/harmony-one/harmony/numeric"
	"github.com/pkg/errors"
)

// Parameters - represents the test case tx parameters
type Parameters struct {
	SenderCount   int64 `yaml:"sender_count"`
	Senders       []sdkAccounts.Account
	ReceiverCount int64 `yaml:"receiver_count"`
	Receivers     []sdkAccounts.Account
	RPCPrefix     string              `yaml:"rpc_prefix,omitempty"`
	FromShardID   uint32              `yaml:"from_shard_id"`
	ToShardID     uint32              `yaml:"to_shard_id"`
	Data          string              `yaml:"data"`
	DataSize      int                 `yaml:"data_size,omitempty"`
	RawAmount     string              `yaml:"amount"`
	Amount        numeric.Dec         `yaml:"-"`
	Gas           sdkNetworkTypes.Gas `yaml:"gas"`
	Nonce         int                 `yaml:"nonce"`
	Count         int                 `yaml:"count"`
	Timeout       int                 `yaml:"timeout"`
}

// Initialize - initializes and converts values for regular test case parameters
func (params *Parameters) Initialize() error {
	if params.RPCPrefix == "" {
		params.RPCPrefix = "hmy"
	}

	decAmount, err := common.NewDecFromString(params.RawAmount)
	if err != nil {
		return errors.Wrapf(err, "Parameters: Amount")
	}
	params.Amount = decAmount

	// This allocates unnecessary data on the heap
	/*if params.DataSize > 0 {
		params.Data = sdkTxs.GenerateTxData(params.Data, params.DataSize)
	}*/

	//Some test cases target shards that aren't available on localnet - switch those test cases to use the highest available shard on localnet (typically 1)
	if params.FromShardID > uint32(config.Configuration.Network.Shards-1) {
		params.FromShardID = uint32(config.Configuration.Network.Shards - 1)
	}

	if params.ToShardID > uint32(config.Configuration.Network.Shards-1) {
		params.ToShardID = uint32(config.Configuration.Network.Shards - 1)
	}

	// Setup gas values
	if err := params.Gas.Initialize(); err != nil {
		return err
	}

	return nil
}

// GenerateTxData - generate tx data on the fly instead of allocating it
func (params *Parameters) GenerateTxData() string {
	if params.DataSize > 0 {
		return sdkTxs.GenerateTxData(params.Data, params.DataSize)
	}

	return ""
}
