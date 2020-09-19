package redelegate

import (
	"fmt"
	"time"

	sdkNetworkTypes "github.com/harmony-one/go-lib/network/types/network"
	sdkDelegation "github.com/harmony-one/go-lib/staking/delegation"
	sdkTxs "github.com/harmony-one/go-lib/transactions"
	"github.com/harmony-one/harmony-tf/accounts"
	"github.com/harmony-one/harmony-tf/config"
	"github.com/harmony-one/harmony-tf/funding"
	"github.com/harmony-one/harmony-tf/logger"
	"github.com/harmony-one/harmony-tf/staking"
	"github.com/harmony-one/harmony-tf/testing"
	testParams "github.com/harmony-one/harmony-tf/testing/parameters"
)

// LockedTokensScenario - initial 1000 ONE delegation, undelegate X amount, delegate X amount after 1 epoch
// Delegator account balance should be set to have enough for only the first delegation
// The second delegation should be equal to or smaller than the undelegation amount
func NextEpochScenario(testCase *testing.TestCase) {
	testing.Title(testCase, "header", testCase.Verbose)
	testCase.Executed = true
	testCase.StartedAt = time.Now().UTC()

	if testCase.ErrorOccurred(nil) {
		return
	}

	requiredFunding := testCase.StakingParameters.Create.Validator.Amount.Add(testCase.StakingParameters.Delegation.Amount)
	fundingMultiple := int64(1)
	_, _, err := funding.CalculateFundingDetails(requiredFunding, fundingMultiple, 0)
	if testCase.ErrorOccurred(err) {
		return
	}

	validatorName := accounts.GenerateTestCaseAccountName(testCase.Name, "Validator")
	account, validator, err := staking.ReuseOrCreateValidator(testCase, validatorName)
	if err != nil {
		msg := fmt.Sprintf("Failed to create validator using account %s", validatorName)
		testCase.HandleError(err, account, msg)
		return
	}

	if validator.Exists {
		delegatorName := accounts.GenerateTestCaseAccountName(testCase.Name, "Delegator")
		delegatorAccount, err := testing.GenerateAndFundAccount(testCase, delegatorName, testCase.StakingParameters.Delegation.Amount, fundingMultiple)
		if err != nil {
			msg := fmt.Sprintf("Failed to fetch latest account balance for the account %s, address: %s", delegatorAccount.Name, delegatorAccount.Address)
			testCase.HandleError(err, &delegatorAccount, msg)
			return
		}

		// Create StakingParams for 1 initial delegation of 1000 ONE
		defaultGasParams := sdkNetworkTypes.Gas{
			RawCost:  "",
			Limit: 0,
			RawPrice: "",
		}
		defaultGasParams.Initialize()
		initialDelegationParams := testParams.DelegationParameters{
			Delegate: testParams.DelegationInstruction{
				RawAmount: "2000",
				Gas:       defaultGasParams,
			},
		}
		initialDelegationParams.Initialize()
		initialDelegationStakingParams := testParams.StakingParameters{
			FromShardID:            0,
			ToShardID:              0,
			Count:                  1,
			Delegation:             initialDelegationParams,
			Mode:                   "",
			ReuseExistingValidator: false,
			Gas:                    defaultGasParams,
			Nonce:                  -1,
			Timeout:                60,
		}

		// Duplicate code from staking.BasicDelegation to create logs & validator initial delegation transaction from generated StakingParams
		// All testCase.StakingParams should be replaced with created initialDelegationStakingParams
		logger.StakingLog("Proceeding to perform delegation...", testCase.Verbose)
		logger.TransactionLog(fmt.Sprintf("Sending delegation transaction - will wait up to %d seconds for it to finalize", testCase.StakingParameters.Timeout), testCase.Verbose)
		initialDelegationTx, err := staking.Delegate(&delegatorAccount, validator.Account, nil, &initialDelegationStakingParams)
		if err != nil {
			msg := fmt.Sprintf("Failed initial delegation from account %s, address %s to validator %s, address %s", delegatorAccount.Name, delegatorAccount.Address, validator.Account.Name, validator.Account.Address)
			testCase.HandleError(err, validator.Account, msg)
			return
		}
		tx := sdkTxs.ToTransaction(delegatorAccount.Address, initialDelegationStakingParams.FromShardID, validator.Account.Address, initialDelegationStakingParams.FromShardID, initialDelegationTx, err)
		txResultColoring := logger.ResultColoring(tx.Success, true)
		logger.TransactionLog(fmt.Sprintf("Performed delegation - transaction hash: %s, tx successful: %s", tx.TransactionHash, txResultColoring), testCase.Verbose)

		node := config.Configuration.Network.API.NodeAddress(initialDelegationStakingParams.FromShardID)
		delegations, err := sdkDelegation.ByDelegator(node, delegatorAccount.Address)
		if err != nil {
			msg := fmt.Sprintf("Failed initial delegation from account %s, address %s to validator %s, address %s", delegatorAccount.Name, delegatorAccount.Address, validator.Account.Name, validator.Account.Address)
			testCase.HandleError(err, validator.Account, msg)
			return
		}

		delegationSucceeded := false
		for _, del := range delegations {
			if del.DelegatorAddress == delegatorAccount.Address && del.ValidatorAddress == validator.Account.Address {
				delegationSucceeded = true
				break
			}
		}

		delegationSucceededColoring := logger.ResultColoring(delegationSucceeded, true)
		logger.StakingLog(fmt.Sprintf("Initial delegation from %s to %s of %f, successful: %s", delegatorAccount.Address, validator.Account.Address, initialDelegationStakingParams.Delegation.Delegate.Amount, delegationSucceededColoring), testCase.Verbose)

		testCase.Transactions = append(testCase.Transactions, tx)

		// Undelegation
		undelegationTx, _, err := staking.BasicUndelegation(testCase, &delegatorAccount, validator.Account, nil)
		if err != nil {
			msg := fmt.Sprintf("Failed to undelegate from account %s, address %s to validator %s, address %s", delegatorAccount.Name, delegatorAccount.Address, validator.Account.Name, validator.Account.Address)
			testCase.HandleError(err, validator.Account, msg)
			return
		}

		testCase.Transactions = append(testCase.Transactions, undelegationTx)

		// TODO: Use param for number of retries
		// TODO: Check transaction accepted epoch instead of current
		var currentEpoch uint32
		for i := 0; i < 5; i++ {
			currentEpoch, err = config.Configuration.Network.API.CurrentEpoch(testCase.StakingParameters.FromShardID)
			if err != nil {
				// Maybe should log errors
				continue
			}
			logger.Log(fmt.Sprintf("Current Epoch: %d", currentEpoch), true)
			break
		}
		// TODO: Add timeout to this loop (5.5 minutes?)
		for true {
			epochCheck, err := config.Configuration.Network.API.CurrentEpoch(testCase.StakingParameters.FromShardID)
			if err != nil {
				// Maybe also log errors here
				continue
			}
			if epochCheck > currentEpoch {
				break
			}
			time.Sleep(time.Duration(20) * time.Second)
		}
		logger.Log("Reach next epoch. Attempting redelegation transactions.", true)

		// Redelegation
		delegationTx, delegationSucceeded, err := staking.BasicDelegation(testCase, &delegatorAccount, validator.Account, nil)
		if err != nil {
			msg := fmt.Sprintf("Failed to delegate from account %s, address %s to validator %s, address %s", delegatorAccount.Name, delegatorAccount.Address, validator.Account.Name, validator.Account.Address)
			testCase.HandleError(err, validator.Account, msg)
			return
		}
		testCase.Transactions = append(testCase.Transactions, delegationTx)

		testCase.Result = delegationTx.Success && delegationSucceeded

		logger.TeardownLog("Performing test teardown (returning funds and removing accounts)", testCase.Verbose)
		testing.Teardown(&delegatorAccount, testCase.StakingParameters.FromShardID, config.Configuration.Funding.Account.Address, testCase.StakingParameters.FromShardID)
	}

	if !testCase.StakingParameters.ReuseExistingValidator {
		testing.Teardown(validator.Account, testCase.StakingParameters.FromShardID, config.Configuration.Funding.Account.Address, testCase.StakingParameters.FromShardID)
	}

	logger.ResultLog(testCase.Result, testCase.Expected, testCase.Verbose)
	testing.Title(testCase, "footer", testCase.Verbose)

	testCase.FinishedAt = time.Now().UTC()
}
