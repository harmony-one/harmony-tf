package staking

import (
	"errors"

	sdkAccounts "github.com/harmony-one/go-lib/accounts"
	sdkCrypto "github.com/harmony-one/go-lib/crypto"
	sdkNetworkNonce "github.com/harmony-one/go-lib/network/rpc/nonces"
	sdkValidator "github.com/harmony-one/go-lib/staking/validator"
	"github.com/harmony-one/harmony-tf/config"
	testParams "github.com/harmony-one/harmony-tf/testing/parameters"
	"github.com/harmony-one/harmony/numeric"
)

var (
	errNilAmount             = errors.New("Amount can not be nil")
	errNilMinSelfDelegation  = errors.New("MinSelfDelegation can not be nil")
	errNilMaxTotalDelegation = errors.New("MaxTotalDelegation can not be nil")
)

// CreateValidator - creates a validator
func CreateValidator(validatorAccount *sdkAccounts.Account, senderAccount *sdkAccounts.Account, params *testParams.StakingParameters, blsKeys []sdkCrypto.BLSKey) (map[string]interface{}, error) {
	if err := validateValidatorValues(params.Create.Validator); err != nil {
		return nil, err
	}

	if senderAccount == nil {
		senderAccount = validatorAccount
	}
	senderAccount.Unlock()

	if params.Create.Validator.Account == nil {
		params.Create.Validator.Account = validatorAccount
	}

	rpcClient, err := config.Configuration.Network.API.RPCClient(params.FromShardID)
	if err != nil {
		return nil, err
	}

	var currentNonce uint64
	if params.Nonce < 0 {
		currentNonce = sdkNetworkNonce.CurrentNonce(rpcClient, senderAccount.Address)
		if err != nil {
			return nil, err
		}
	} else {
		currentNonce = uint64(params.Nonce)
	}

	txResult, err := sdkValidator.Create(
		senderAccount.Keystore,
		senderAccount.Account,
		rpcClient,
		config.Configuration.Network.API.ChainID,
		validatorAccount.Address,
		params.Create.Validator.ToStakingDescription(),
		params.Create.Validator.ToCommissionRates(),
		params.Create.Validator.MinimumSelfDelegation,
		params.Create.Validator.MaximumTotalDelegation,
		blsKeys,
		params.Create.Validator.Amount,
		params.Gas.Limit,
		params.Gas.Price,
		currentNonce,
		config.Configuration.Account.Passphrase,
		config.Configuration.Network.API.NodeAddress(params.FromShardID),
		params.Timeout,
	)

	if err != nil {
		return nil, err
	}

	return txResult, nil
}

// EditValidator - edits a given validator using the provided information
func EditValidator(validatorAccount *sdkAccounts.Account, senderAccount *sdkAccounts.Account, params *testParams.StakingParameters, blsKeyToRemove *sdkCrypto.BLSKey, blsKeyToAdd *sdkCrypto.BLSKey) (map[string]interface{}, error) {
	if senderAccount == nil {
		senderAccount = validatorAccount
	}
	senderAccount.Unlock()

	if params.Edit.Validator.Account == nil {
		params.Edit.Validator.Account = validatorAccount
	}

	rpcClient, err := config.Configuration.Network.API.RPCClient(params.FromShardID)
	if err != nil {
		return nil, err
	}

	var currentNonce uint64
	if params.Nonce < 0 {
		currentNonce = sdkNetworkNonce.CurrentNonce(rpcClient, senderAccount.Address)
		if err != nil {
			return nil, err
		}
	} else {
		currentNonce = uint64(params.Nonce)
	}

	var commissionRate *numeric.Dec
	if !params.Edit.Validator.Commission.Rate.IsNil() {
		commissionRate = &params.Edit.Validator.Commission.Rate
	}

	gasLimit := params.Gas.Limit
	gasPrice := params.Gas.Price

	if params.Edit.Gas.RawPrice != "" {
		gasLimit = params.Edit.Gas.Limit
		gasPrice = params.Edit.Gas.Price
	}

	txResult, err := sdkValidator.Edit(
		senderAccount.Keystore,
		senderAccount.Account,
		rpcClient,
		config.Configuration.Network.API.ChainID,
		validatorAccount.Address,
		params.Edit.Validator.ToStakingDescription(),
		commissionRate,
		params.Edit.Validator.MinimumSelfDelegation,
		params.Edit.Validator.MaximumTotalDelegation,
		blsKeyToRemove,
		blsKeyToAdd,
		params.Edit.Validator.EligibilityStatus,
		gasLimit,
		gasPrice,
		currentNonce,
		config.Configuration.Account.Passphrase,
		config.Configuration.Network.API.NodeAddress(params.FromShardID),
		params.Timeout,
	)

	if err != nil {
		return nil, err
	}

	return txResult, nil
}

// DisableValidator - disables a given validatora
func DisableValidator(validatorAccount *sdkAccounts.Account, params *testParams.StakingParameters) (map[string]interface{}, error) {
	return EditValidatorStatus(validatorAccount, nil, params, "inactive")
}

// EditValidatorStatus - edits a given validator's status
func EditValidatorStatus(validatorAccount *sdkAccounts.Account, senderAccount *sdkAccounts.Account, params *testParams.StakingParameters, status string) (map[string]interface{}, error) {
	if senderAccount == nil {
		senderAccount = validatorAccount
	}
	senderAccount.Unlock()

	if params.Edit.Validator.Account == nil {
		params.Edit.Validator.Account = validatorAccount
	}

	rpcClient, err := config.Configuration.Network.API.RPCClient(params.FromShardID)
	if err != nil {
		return nil, err
	}

	var currentNonce uint64
	if params.Nonce < 0 {
		currentNonce = sdkNetworkNonce.CurrentNonce(rpcClient, senderAccount.Address)
		if err != nil {
			return nil, err
		}
	} else {
		currentNonce = uint64(params.Nonce)
	}

	gasLimit := params.Gas.Limit
	gasPrice := params.Gas.Price

	if params.Edit.Gas.RawPrice != "" {
		gasLimit = params.Edit.Gas.Limit
		gasPrice = params.Edit.Gas.Price
	}

	txResult, err := sdkValidator.EditStatus(
		senderAccount.Keystore,
		senderAccount.Account,
		rpcClient,
		config.Configuration.Network.API.ChainID,
		validatorAccount.Address,
		status,
		gasLimit,
		gasPrice,
		currentNonce,
		config.Configuration.Account.Passphrase,
		config.Configuration.Network.API.NodeAddress(params.FromShardID),
		params.Timeout,
	)

	if err != nil {
		return nil, err
	}

	return txResult, nil
}

func validateValidatorValues(validator sdkValidator.Validator) error {
	if validator.Amount.IsNil() {
		return errNilAmount
	}

	if validator.MinimumSelfDelegation.IsNil() {
		return errNilMinSelfDelegation
	}

	if validator.MaximumTotalDelegation.IsNil() {
		return errNilMaxTotalDelegation
	}

	return nil
}
