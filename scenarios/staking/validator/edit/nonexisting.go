package edit

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

// NonExistingScenario - executes an edit validator test case using a non-existing validator
func NonExistingScenario(testCase *testing.TestCase) {
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
		msg := fmt.Sprintf("Failed to fetch latest account balance for the account %s, address: %s", account.Name, account.Address)
		testCase.HandleError(err, &account, msg)
		return
	}

	testCase.StakingParameters.Create.Validator.Account = &account
	tx, err := staking.BasicEditValidator(testCase, &account, nil, nil, nil)
	if err != nil {
		msg := fmt.Sprintf("Failed to edit validator using account %s, address: %s", account.Name, account.Address)
		testCase.HandleError(err, &account, msg)
		return
	}
	testCase.Transactions = append(testCase.Transactions, tx)

	testCase.Result = tx.Success

	logger.TeardownLog("Performing test teardown (returning funds and removing accounts)", testCase.Verbose)
	logger.ResultLog(testCase.Result, testCase.Expected, testCase.Verbose)
	testing.Title(testCase, "footer", testCase.Verbose)

	staking.DisableValidator(&account, &testCase.StakingParameters)
	testing.Teardown(&account, testCase.StakingParameters.FromShardID, config.Configuration.Funding.Account.Address, testCase.StakingParameters.FromShardID)
	testCase.FinishedAt = time.Now().UTC()
}
