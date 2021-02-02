package create

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

// ExistingBLSKeyScenario - executes a create validator test case using a previously used BLS key
func ExistingBLSKeyScenario(testCase *testing.TestCase) {
	testing.Title(testCase, "header", testCase.Verbose)
	testCase.Executed = true
	testCase.StartedAt = time.Now().UTC()

	if testCase.ErrorOccurred(nil) {
		return
	}

	fundingMultiple := int64(1)
	_, _, err := funding.CalculateFundingDetails(testCase.StakingParameters.Create.Validator.Amount, fundingMultiple, 0)
	if testCase.ErrorOccurred(err) {
		return
	}

	validatorName := accounts.GenerateTestCaseAccountName(testCase.Name, "Validator")
	account, err := testing.GenerateAndFundAccount(testCase, validatorName, testCase.StakingParameters.Create.Validator.Amount, fundingMultiple)
	if err != nil {
		msg := fmt.Sprintf("Failed to generate and fund the account %s", validatorName)
		testCase.HandleError(err, &account, msg)
		return
	}

	testCase.StakingParameters.Create.Validator.Account = &account
	tx, blsKeys, validatorExists, err := staking.BasicCreateValidator(testCase, &account, nil, nil)
	if err != nil {
		msg := fmt.Sprintf("Failed to create validator using account %s, address: %s", account.Name, account.Address)
		testCase.HandleError(err, &account, msg)
		return
	}
	testCase.Transactions = append(testCase.Transactions, tx)

	if tx.Success && validatorExists {
		logger.StakingLog(fmt.Sprintf("Proceeding with trying to create a new validator using the previously used bls key: %s", blsKeys[0].PublicKeyHex), testCase.Verbose)

		duplicateValidatorName := accounts.GenerateTestCaseAccountName(testCase.Name, "Validator_DuplicateBLSKey")
		duplicateAccount, err := testing.GenerateAndFundAccount(testCase, duplicateValidatorName, testCase.StakingParameters.Create.Validator.Amount, 1)
		if err != nil {
			msg := fmt.Sprintf("Failed to generate and fund account: %s", duplicateValidatorName)
			testCase.HandleError(err, &duplicateAccount, msg)
			return
		}

		testCase.StakingParameters.Create.Validator.Account = &duplicateAccount
		duplicateTx, _, duplicateValidatorExists, err := staking.BasicCreateValidator(testCase, &duplicateAccount, nil, blsKeys)
		if err != nil {
			msg := fmt.Sprintf("Failed to create validator using account %s, address: %s", duplicateAccount.Name, duplicateAccount.Address)
			testCase.HandleError(err, &account, msg)
			return
		}
		testCase.Transactions = append(testCase.Transactions, duplicateTx)

		testCase.Result = duplicateTx.Success && duplicateValidatorExists
		testing.Teardown(&duplicateAccount, testCase.StakingParameters.FromShardID, config.Configuration.Funding.Account.Address, testCase.StakingParameters.FromShardID)
	}

	logger.TeardownLog("Performing test teardown (returning funds and removing accounts)", testCase.Verbose)
	logger.ResultLog(testCase.Result, testCase.Expected, testCase.Verbose)
	testing.Title(testCase, "footer", testCase.Verbose)

	staking.DisableValidator(&account, &testCase.StakingParameters)
	testing.Teardown(&account, testCase.StakingParameters.FromShardID, config.Configuration.Funding.Account.Address, testCase.StakingParameters.FromShardID)

	testCase.FinishedAt = time.Now().UTC()
}
