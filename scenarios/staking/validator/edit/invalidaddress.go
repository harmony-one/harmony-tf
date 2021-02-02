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

// InvalidAddressScenario - executes an edit validator test case using an invalid address
func InvalidAddressScenario(testCase *testing.TestCase) {
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
	account, validator, err := staking.ReuseOrCreateValidator(testCase, validatorName)
	if err != nil {
		msg := fmt.Sprintf("Failed to create validator using account %s", validatorName)
		testCase.HandleError(err, account, msg)
		return
	}

	if validator.Exists {
		invalidAccountName := accounts.GenerateTestCaseAccountName(testCase.Name, "InvalidChanger")
		invalidAccount, err := testing.GenerateAndFundAccount(testCase, invalidAccountName, testCase.StakingParameters.Create.Validator.Amount, fundingMultiple)
		if err != nil {
			msg := fmt.Sprintf("Failed to generate and fund account %s", invalidAccountName)
			testCase.HandleError(err, validator.Account, msg)
			return
		}

		lastEditTx, lastEditTxErr := staking.BasicEditValidator(testCase, validator.Account, &invalidAccount, nil, nil)
		if lastEditTxErr != nil {
			msg := fmt.Sprintf("Failed to edit validator using account %s, address: %s", invalidAccount.Name, invalidAccount.Address)
			testCase.HandleError(lastEditTxErr, validator.Account, msg)
			return
		}
		testCase.Transactions = append(testCase.Transactions, lastEditTx)

		testCase.Result = lastEditTx.Success

		logger.TeardownLog("Performing test teardown (returning funds and removing accounts)", testCase.Verbose)

		testing.Teardown(&invalidAccount, testCase.StakingParameters.FromShardID, config.Configuration.Funding.Account.Address, testCase.StakingParameters.FromShardID)
		if !testCase.StakingParameters.ReuseExistingValidator {
			staking.DisableValidator(validator.Account, &testCase.StakingParameters)
			testing.Teardown(validator.Account, testCase.StakingParameters.FromShardID, config.Configuration.Funding.Account.Address, testCase.StakingParameters.FromShardID)
		}
	}

	logger.ResultLog(testCase.Result, testCase.Expected, testCase.Verbose)
	testing.Title(testCase, "footer", testCase.Verbose)

	testCase.FinishedAt = time.Now().UTC()
}
