package delegate

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

// StandardScenario - executes a standard delegation test case
func StandardScenario(testCase *testing.TestCase) {
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

		delegationTx, delegationSucceeded, err := staking.BasicDelegation(testCase, &delegatorAccount, validator.Account, nil)
		if err != nil {
			msg := fmt.Sprintf("Failed to delegate from account %s, address %s to validator %s, address: %s", delegatorAccount.Name, delegatorAccount.Address, validator.Account.Name, validator.Account.Address)
			testCase.HandleError(err, validator.Account, msg)
			return
		}
		testCase.Transactions = append(testCase.Transactions, delegationTx)

		testCase.Result = delegationTx.Success && delegationSucceeded

		logger.TeardownLog("Performing test teardown (returning funds and removing accounts)", testCase.Verbose)
		testing.Teardown(&delegatorAccount, testCase.StakingParameters.FromShardID, config.Configuration.Funding.Account.Address, testCase.StakingParameters.FromShardID)
	}

	if !testCase.StakingParameters.ReuseExistingValidator {
		staking.DisableValidator(validator.Account, &testCase.StakingParameters)
		testing.Teardown(validator.Account, testCase.StakingParameters.FromShardID, config.Configuration.Funding.Account.Address, testCase.StakingParameters.FromShardID)
	}

	logger.ResultLog(testCase.Result, testCase.Expected, testCase.Verbose)
	testing.Title(testCase, "footer", testCase.Verbose)

	testCase.FinishedAt = time.Now().UTC()
}
