package staking

import (
	"fmt"
	"time"

	"github.com/harmony-one/harmony-tf/balances"
	"github.com/harmony-one/harmony-tf/config"
	"github.com/harmony-one/harmony-tf/crypto"
	"github.com/harmony-one/harmony-tf/logger"
	"github.com/harmony-one/harmony-tf/testing"
	"github.com/harmony-one/harmony/numeric"

	sdkAccounts "github.com/harmony-one/go-lib/accounts"
	sdkCrypto "github.com/harmony-one/go-lib/crypto"
	sdkDelegation "github.com/harmony-one/go-lib/staking/delegation"
	sdkValidator "github.com/harmony-one/go-lib/staking/validator"
	sdkTxs "github.com/harmony-one/go-lib/transactions"
)

// ReuseOrCreateValidator - reuse an existing validator or create a new one
func ReuseOrCreateValidator(testCase *testing.TestCase, validatorName string) (account *sdkAccounts.Account, validator *sdkValidator.Validator, err error) {
	if testCase.StakingParameters.ReuseExistingValidator && config.Configuration.Framework.CurrentValidator != nil {
		return config.Configuration.Framework.CurrentValidator.Account, config.Configuration.Framework.CurrentValidator, nil
	}

	validator = &testCase.StakingParameters.Create.Validator
	acc, err := testing.GenerateAndFundAccount(testCase, validatorName, testCase.StakingParameters.Create.Validator.Amount, 1)
	if err != nil {
		return nil, nil, err
	}

	account = &acc
	validator.Account = account
	testCase.StakingParameters.Create.Validator.Account = account

	tx, createdBlsKeys, validatorExists, err := BasicCreateValidator(testCase, validator.Account, nil, nil)
	if err != nil {
		return account, nil, err
	}
	testCase.Transactions = append(testCase.Transactions, tx)
	testCase.StakingParameters.Create.Validator.BLSKeys = createdBlsKeys

	if config.Configuration.Network.StakingWaitTime > 0 {
		time.Sleep(time.Duration(config.Configuration.Network.StakingWaitTime) * time.Second)
	}

	validator.Exists = validatorExists
	validator.BLSKeys = createdBlsKeys

	// The ending balance of the account that created the validator should be less than the funded amount since the create validator tx should've used the specified amount for self delegation
	accountEndingBalance, _ := balances.GetShardBalance(validator.Account.Address, testCase.StakingParameters.FromShardID)
	expectedAccountEndingBalance := account.Balance.Sub(testCase.StakingParameters.Create.Validator.Amount)

	if testCase.Expected {
		logger.BalanceLog(fmt.Sprintf("Account %s, address: %s has an ending balance of %f in shard %d after creating the validator - expected value: %f (or less)", validator.Account.Name, validator.Account.Address, accountEndingBalance, testCase.StakingParameters.FromShardID, expectedAccountEndingBalance), testCase.Verbose)
	} else {
		logger.BalanceLog(fmt.Sprintf("Account %s, address: %s has an ending balance of %f in shard %d after creating the validator", validator.Account.Name, validator.Account.Address, accountEndingBalance, testCase.StakingParameters.FromShardID), testCase.Verbose)
	}

	if testCase.StakingParameters.ReuseExistingValidator && config.Configuration.Framework.CurrentValidator == nil && validatorExists {
		config.Configuration.Framework.CurrentValidator = validator
		validator = config.Configuration.Framework.CurrentValidator
	}

	return account, validator, nil
}

// BasicCreateValidator - helper method to create a validator
func BasicCreateValidator(testCase *testing.TestCase, validatorAccount *sdkAccounts.Account, senderAccount *sdkAccounts.Account, blsKeys []sdkCrypto.BLSKey) (sdkTxs.Transaction, []sdkCrypto.BLSKey, bool, error) {
	if senderAccount == nil {
		senderAccount = validatorAccount
	}

	if blsKeys == nil || len(blsKeys) == 0 {
		blsKeys = crypto.GenerateBlsKeys(testCase.StakingParameters.Create.BLSKeyCount, testCase.StakingParameters.Create.Validator.ShardID, testCase.StakingParameters.Create.BLSSignatureMessage)
	}

	switch testCase.StakingParameters.Mode {
	case "duplicate_bls_key", "duplicateblskey":
		blsKeys = append(blsKeys, blsKeys[0])
	case "amount_larger_than_balance", "amountlargerthanbalance":
		testCase.StakingParameters.Create.Validator.Amount = testCase.StakingParameters.Create.Validator.Amount.Mul(numeric.NewDec(2))
	}

	if len(blsKeys) > 0 {
		for _, blsKey := range blsKeys {
			logger.StakingLog(fmt.Sprintf("Using BLS key %s to create the validator %s", blsKey.PublicKeyHex, validatorAccount.Address), testCase.Verbose)
		}
	}

	logger.TransactionLog(fmt.Sprintf("Sending create validator transaction - will wait up to %d seconds for it to finalize", testCase.StakingParameters.Timeout), testCase.Verbose)

	rawTx, err := CreateValidator(validatorAccount, senderAccount, &testCase.StakingParameters, blsKeys)
	if err != nil {
		return sdkTxs.Transaction{}, nil, false, err
	}

	tx := sdkTxs.ToTransaction(senderAccount.Address, testCase.StakingParameters.FromShardID, senderAccount.Address, testCase.StakingParameters.FromShardID, rawTx, err)
	txResultColoring := logger.ResultColoring(tx.Success, true)
	logger.TransactionLog(fmt.Sprintf("Performed create validator - address: %s - transaction hash: %s, tx successful: %s", validatorAccount.Address, tx.TransactionHash, txResultColoring), testCase.Verbose)

	rpcClient, err := config.Configuration.Network.API.RPCClient(testCase.StakingParameters.FromShardID)
	validatorExists := sdkValidator.Exists(rpcClient, validatorAccount.Address)
	addressExistsColoring := logger.ResultColoring(validatorExists, true)
	logger.StakingLog(fmt.Sprintf("Validator with address %s exists: %s", validatorAccount.Address, addressExistsColoring), testCase.Verbose)

	return tx, blsKeys, validatorExists, nil
}

// BasicEditValidator - helper method to edit a validator
func BasicEditValidator(testCase *testing.TestCase, validatorAccount *sdkAccounts.Account, senderAccount *sdkAccounts.Account, blsKeyToRemove *sdkCrypto.BLSKey, blsKeyToAdd *sdkCrypto.BLSKey) (sdkTxs.Transaction, error) {
	if senderAccount == nil {
		senderAccount = validatorAccount
	}

	logger.StakingLog(fmt.Sprintf("Proceeding to edit the validator %s ...", validatorAccount.Address), testCase.Verbose)
	testCase.StakingParameters.Edit.DetectChanges(testCase.Verbose)
	logger.TransactionLog(fmt.Sprintf("Sending edit validator transaction - will wait up to %d seconds for it to finalize", testCase.StakingParameters.Timeout), testCase.Verbose)

	editRawTx, err := EditValidator(validatorAccount, senderAccount, &testCase.StakingParameters, blsKeyToRemove, blsKeyToAdd)
	if err != nil {
		return sdkTxs.Transaction{}, err
	}
	editTx := sdkTxs.ToTransaction(senderAccount.Address, testCase.StakingParameters.FromShardID, senderAccount.Address, testCase.StakingParameters.FromShardID, editRawTx, err)
	editTxResultColoring := logger.ResultColoring(editTx.Success, true)
	logger.TransactionLog(fmt.Sprintf("Performed edit validator - transaction hash: %s, tx successful: %s", editTx.TransactionHash, editTxResultColoring), testCase.Verbose)

	return editTx, nil
}

// BasicEditValidatorStatus - helper method to edit a validator
func BasicEditValidatorStatus(testCase *testing.TestCase, validatorAccount *sdkAccounts.Account, senderAccount *sdkAccounts.Account, status string) (sdkTxs.Transaction, error) {
	if senderAccount == nil {
		senderAccount = validatorAccount
	}

	logger.StakingLog(fmt.Sprintf("Proceeding to edit the the status for validator %s ...", validatorAccount.Address), testCase.Verbose)
	logger.TransactionLog(fmt.Sprintf("Sending edit validator status transaction - will wait up to %d seconds for it to finalize", testCase.StakingParameters.Timeout), testCase.Verbose)

	editRawTx, err := EditValidatorStatus(validatorAccount, senderAccount, &testCase.StakingParameters, status)
	if err != nil {
		return sdkTxs.Transaction{}, err
	}
	editTx := sdkTxs.ToTransaction(senderAccount.Address, testCase.StakingParameters.FromShardID, senderAccount.Address, testCase.StakingParameters.FromShardID, editRawTx, err)
	editTxResultColoring := logger.ResultColoring(editTx.Success, true)
	logger.TransactionLog(fmt.Sprintf("Performed edit validator status - transaction hash: %s, tx successful: %s", editTx.TransactionHash, editTxResultColoring), testCase.Verbose)

	return editTx, nil
}

// BasicDelegation - helper method to perform delegation
func BasicDelegation(testCase *testing.TestCase, delegatorAccount *sdkAccounts.Account, validatorAccount *sdkAccounts.Account, senderAccount *sdkAccounts.Account) (sdkTxs.Transaction, bool, error) {
	logger.StakingLog("Proceeding to perform delegation...", testCase.Verbose)
	logger.TransactionLog(fmt.Sprintf("Sending delegation transaction - will wait up to %d seconds for it to finalize", testCase.StakingParameters.Timeout), testCase.Verbose)

	rawTx, err := Delegate(delegatorAccount, validatorAccount, senderAccount, &testCase.StakingParameters)
	if err != nil {
		return sdkTxs.Transaction{}, false, err
	}
	tx := sdkTxs.ToTransaction(delegatorAccount.Address, testCase.StakingParameters.FromShardID, validatorAccount.Address, testCase.StakingParameters.FromShardID, rawTx, err)
	txResultColoring := logger.ResultColoring(tx.Success, true)
	logger.TransactionLog(fmt.Sprintf("Performed delegation - transaction hash: %s, tx successful: %s", tx.TransactionHash, txResultColoring), testCase.Verbose)

	node := config.Configuration.Network.API.NodeAddress(testCase.StakingParameters.FromShardID)
	delegations, err := sdkDelegation.ByDelegator(node, delegatorAccount.Address)
	if err != nil {
		return sdkTxs.Transaction{}, false, err
	}

	delegationSucceeded := false
	for _, del := range delegations {
		if del.DelegatorAddress == delegatorAccount.Address && del.ValidatorAddress == validatorAccount.Address {
			delegationSucceeded = true
			break
		}
	}

	delegationSucceededColoring := logger.ResultColoring(delegationSucceeded, true)
	logger.StakingLog(fmt.Sprintf("Delegation from %s to %s of %f, successful: %s", delegatorAccount.Address, validatorAccount.Address, testCase.StakingParameters.Delegation.Delegate.Amount, delegationSucceededColoring), testCase.Verbose)

	return tx, delegationSucceeded, nil
}

// BasicUndelegation - helper method to perform undelegation
func BasicUndelegation(testCase *testing.TestCase, delegatorAccount *sdkAccounts.Account, validatorAccount *sdkAccounts.Account, senderAccount *sdkAccounts.Account) (sdkTxs.Transaction, bool, error) {
	logger.StakingLog("Proceeding to perform undelegation...", testCase.Verbose)
	logger.TransactionLog(fmt.Sprintf("Sending undelegation transaction - will wait up to %d seconds for it to finalize", testCase.StakingParameters.Timeout), testCase.Verbose)

	rawTx, err := Undelegate(delegatorAccount, validatorAccount, senderAccount, &testCase.StakingParameters)
	if err != nil {
		return sdkTxs.Transaction{}, false, err
	}
	tx := sdkTxs.ToTransaction(delegatorAccount.Address, testCase.StakingParameters.FromShardID, validatorAccount.Address, testCase.StakingParameters.FromShardID, rawTx, err)
	txResultColoring := logger.ResultColoring(tx.Success, true)
	logger.TransactionLog(fmt.Sprintf("Performed undelegation - transaction hash: %s, tx successful: %s", tx.TransactionHash, txResultColoring), testCase.Verbose)

	node := config.Configuration.Network.API.NodeAddress(testCase.StakingParameters.FromShardID)
	delegations, err := sdkDelegation.ByDelegator(node, delegatorAccount.Address)
	if err != nil {
		return sdkTxs.Transaction{}, false, err
	}

	undelegationSucceeded := false
	for _, del := range delegations {
		if len(del.Undelegations) > 0 {
			undelegationSucceeded = true
			break
		}
	}

	undelegationSucceededColoring := logger.ResultColoring(undelegationSucceeded, true)
	logger.StakingLog(fmt.Sprintf("Performed undelegation from validator %s by delegator %s, amount: %f, successful: %s", validatorAccount.Address, delegatorAccount.Address, testCase.StakingParameters.Delegation.Undelegate.Amount, undelegationSucceededColoring), testCase.Verbose)

	return tx, true, nil
}

// ManageBLSKeys - manage bls keys for edit validator scenarios
func ManageBLSKeys(validator *sdkValidator.Validator, mode string, blsSignatureMessage string, verbose bool) (blsKeyToRemove *sdkCrypto.BLSKey, blsKeyToAdd *sdkCrypto.BLSKey, err error) {
	switch mode {
	case "add_bls_key":
		keyToAdd, err := crypto.GenerateBlsKey(validator.ShardID, blsSignatureMessage)
		if err != nil {
			fmt.Printf("\n\nStakingParameters.ManageBLSKeys - err: %+v\n\n", err)
			return nil, nil, err
		}
		blsKeyToAdd = &keyToAdd
		logger.StakingLog(fmt.Sprintf("Adding bls key %v to validator: %s", blsKeyToAdd.PublicKeyHex, validator.Account.Address), verbose)

	case "add_existing_bls_key":
		blsKeyToAdd = &validator.BLSKeys[0]
		logger.StakingLog(fmt.Sprintf("Adding already existing bls key %v to validator: %s", blsKeyToAdd.PublicKeyHex, validator.Account.Address), verbose)

	case "remove_bls_key":
		blsKeyToRemove = &validator.BLSKeys[0]
		logger.StakingLog(fmt.Sprintf("Removing bls key %v from validator: %s", blsKeyToRemove.PublicKeyHex, validator.Account.Address), verbose)

	case "remove_non_existing_bls_key":
		nonExistingKey, err := crypto.GenerateBlsKey(validator.ShardID, blsSignatureMessage)
		if err != nil {
			return nil, nil, err
		}
		blsKeyToRemove = &nonExistingKey
		logger.StakingLog(fmt.Sprintf("Removing non existing bls key %v from validator: %s", blsKeyToRemove.PublicKeyHex, validator.Account.Address), verbose)
	}

	return blsKeyToRemove, blsKeyToAdd, nil
}
