package undelegate

import (
	"fmt"
	"time"

	"github.com/harmony-one/harmony-tf/accounts"
	"github.com/harmony-one/harmony-tf/config"
	"github.com/harmony-one/harmony-tf/funding"
	"github.com/harmony-one/harmony-tf/logger"
	"github.com/harmony-one/harmony-tf/staking"
	"github.com/harmony-one/harmony-tf/testing"
)

// NonExistingScenario - executes a delegation test case where the validator doesn't exist
func NonExistingScenario(testCase *testing.TestCase) {
	testing.Title(testCase, "header", testCase.Verbose)
	testCase.Executed = true
	testCase.StartedAt = time.Now().UTC()

	if testCase.ErrorOccurred(nil) {
		return
	}

	fundingMultiple := int64(1)
	_, _, err := funding.CalculateFundingDetails(testCase.StakingParameters.Delegation.Amount, fundingMultiple, 0)
	if testCase.ErrorOccurred(err) {
		return
	}

	validatorName := accounts.GenerateTestCaseAccountName(testCase.Name, "Validator")
	validatorAccount, err := accounts.GenerateAccount(validatorName)
	if err != nil {
		msg := fmt.Sprintf("Failed to generate account %s", validatorName)
		testCase.HandleError(err, &validatorAccount, msg)
		return
	}

	delegatorName := accounts.GenerateTestCaseAccountName(testCase.Name, "Delegator")
	delegatorAccount, err := testing.GenerateAndFundAccount(testCase, delegatorName, testCase.StakingParameters.Delegation.Amount, fundingMultiple)
	if err != nil {
		msg := fmt.Sprintf("Failed to generate and fund account %s", delegatorName)
		testCase.HandleError(err, &delegatorAccount, msg)
		return
	}

	undelegationTx, undelegationSucceeded, err := staking.BasicUndelegation(testCase, &delegatorAccount, &validatorAccount, nil)
	if err != nil {
		msg := fmt.Sprintf("Failed to undelegate from account %s, address %s to validator %s, address: %s", delegatorAccount.Name, delegatorAccount.Address, validatorAccount.Name, validatorAccount.Address)
		testCase.HandleError(err, &validatorAccount, msg)
		return
	}
	testCase.Transactions = append(testCase.Transactions, undelegationTx)

	testCase.Result = undelegationTx.Success && undelegationSucceeded

	logger.TeardownLog("Performing test teardown (returning funds and removing accounts)", testCase.Verbose)
	logger.ResultLog(testCase.Result, testCase.Expected, testCase.Verbose)
	testing.Title(testCase, "footer", testCase.Verbose)

	if !testCase.StakingParameters.ReuseExistingValidator {
		staking.DisableValidator(&validatorAccount, &testCase.StakingParameters)
		testing.Teardown(&validatorAccount, testCase.StakingParameters.FromShardID, config.Configuration.Funding.Account.Address, testCase.StakingParameters.FromShardID)
	}
	testing.Teardown(&delegatorAccount, testCase.StakingParameters.FromShardID, config.Configuration.Funding.Account.Address, testCase.StakingParameters.FromShardID)

	testCase.FinishedAt = time.Now().UTC()
}
