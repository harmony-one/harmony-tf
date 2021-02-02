package create

import (
	"fmt"
	"time"

	"github.com/harmony-one/harmony-tf/accounts"
	"github.com/harmony-one/harmony-tf/balances"
	"github.com/harmony-one/harmony-tf/config"
	"github.com/harmony-one/harmony-tf/funding"
	"github.com/harmony-one/harmony-tf/logger"
	"github.com/harmony-one/harmony-tf/staking"
	"github.com/harmony-one/harmony-tf/testing"
)

// InvalidAddressScenario - executes a create validator test case where the validator address isn't the same as the account/address sending the create validator transaction
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

	senderName := accounts.GenerateTestCaseAccountName(testCase.Name, "InvalidSender")
	senderAccount, err := testing.GenerateAndFundAccount(testCase, senderName, testCase.StakingParameters.Create.Validator.Amount, fundingMultiple)
	if err != nil {
		msg := fmt.Sprintf("Failed to generate and fund account: %s", senderName)
		testCase.HandleError(err, &senderAccount, msg)
		return
	}

	validatorName := accounts.GenerateTestCaseAccountName(testCase.Name, "InvalidValidator")
	logger.AccountLog(fmt.Sprintf("Generating a new account: %s", validatorName), testCase.Verbose)
	validatorAccount, err := accounts.GenerateAccount(validatorName)
	if err != nil {
		msg := fmt.Sprintf("Failed to generate account %s", validatorName)
		testCase.HandleError(err, &validatorAccount, msg)
		return
	}
	logger.AccountLog(fmt.Sprintf("Generated account: %s, address: %s", validatorAccount.Name, validatorAccount.Address), testCase.Verbose)

	testCase.StakingParameters.Create.Validator.Account = &validatorAccount
	tx, _, validatorExists, err := staking.BasicCreateValidator(testCase, &validatorAccount, &senderAccount, nil)
	if err != nil {
		msg := fmt.Sprintf("Failed to create validator using account %s, address: %s", senderAccount.Name, senderAccount.Address)
		testCase.HandleError(err, &senderAccount, msg)
		return
	}

	testCase.Transactions = append(testCase.Transactions, tx)

	// The ending balance of the account that created the validator should be less than the funded amount since the create validator tx should've used the specified amount for self delegation
	accountEndingBalance, _ := balances.GetShardBalance(senderAccount.Address, testCase.StakingParameters.FromShardID)
	expectedAccountEndingBalance := senderAccount.Balance
	logger.BalanceLog(fmt.Sprintf("Account %s, address: %s has an ending balance of %f in shard %d after the test - expected value: %f (or less)", senderAccount.Name, senderAccount.Address, accountEndingBalance, testCase.StakingParameters.FromShardID, expectedAccountEndingBalance), testCase.Verbose)

	testCase.Result = tx.Success && accountEndingBalance.LT(expectedAccountEndingBalance) && validatorExists

	logger.TeardownLog("Performing test teardown (returning funds and removing accounts)", testCase.Verbose)
	logger.ResultLog(testCase.Result, testCase.Expected, testCase.Verbose)
	testing.Title(testCase, "footer", testCase.Verbose)

	staking.DisableValidator(&validatorAccount, &testCase.StakingParameters)
	testing.Teardown(&senderAccount, testCase.StakingParameters.FromShardID, config.Configuration.Funding.Account.Address, testCase.StakingParameters.FromShardID)

	testCase.FinishedAt = time.Now().UTC()
}
