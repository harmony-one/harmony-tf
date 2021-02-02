package undelegate

import (
	"fmt"
	"strings"
	"time"

	sdkAccounts "github.com/harmony-one/go-lib/accounts"
	tfAccounts "github.com/harmony-one/harmony-tf/accounts"
	"github.com/harmony-one/harmony-tf/balances"
	"github.com/harmony-one/harmony-tf/config"
	"github.com/harmony-one/harmony-tf/funding"
	"github.com/harmony-one/harmony-tf/logger"
	"github.com/harmony-one/harmony-tf/staking"
	"github.com/harmony-one/harmony-tf/testing"
)

// InvalidAddressScenario - executes an undelegation test case where the undelegator address isn't the sender address
func InvalidAddressScenario(testCase *testing.TestCase) {
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

	accounts := map[string]sdkAccounts.Account{}
	accountTypes := []string{
		"validator",
		"delegator",
		"sender",
	}

	for _, accountType := range accountTypes {
		accountName := tfAccounts.GenerateTestCaseAccountName(testCase.Name, strings.Title(accountType))
		account, err := testing.GenerateAndFundAccount(testCase, accountName, testCase.StakingParameters.Create.Validator.Amount, 1)
		if err != nil {
			msg := fmt.Sprintf("Failed to generate and fund %s account %s", accountType, accountName)
			testCase.HandleError(err, &account, msg)
			return
		}
		accounts[accountType] = account
	}

	validatorAccount, delegatorAccount, senderAccount := accounts["validator"], accounts["delegator"], accounts["sender"]
	testCase.StakingParameters.Create.Validator.Account = &validatorAccount
	tx, _, validatorExists, err := staking.BasicCreateValidator(testCase, &validatorAccount, &senderAccount, nil)
	if err != nil {
		msg := fmt.Sprintf("Failed to create validator using account %s, address: %s", validatorAccount.Name, validatorAccount.Address)
		testCase.HandleError(err, &validatorAccount, msg)
		return
	}
	testCase.Transactions = append(testCase.Transactions, tx)

	if config.Configuration.Network.StakingWaitTime > 0 {
		time.Sleep(time.Duration(config.Configuration.Network.StakingWaitTime) * time.Second)
	}

	// The ending balance of the account that created the validator should be less than the funded amount since the create validator tx should've used the specified amount for self delegation
	accountEndingBalance, err := balances.GetShardBalance(validatorAccount.Address, testCase.StakingParameters.FromShardID)
	if err != nil {
		msg := fmt.Sprintf("Failed to fetch ending balance for account %s, address: %s", validatorAccount.Name, validatorAccount.Address)
		testCase.HandleError(err, &validatorAccount, msg)
		return
	}
	expectedAccountEndingBalance := validatorAccount.Balance.Sub(testCase.StakingParameters.Create.Validator.Amount)

	if testCase.Expected {
		logger.BalanceLog(fmt.Sprintf("Account %s, address: %s has an ending balance of %f in shard %d after the test - expected value: %f (or less)", validatorAccount.Name, validatorAccount.Address, accountEndingBalance, testCase.StakingParameters.FromShardID, expectedAccountEndingBalance), testCase.Verbose)
	} else {
		logger.BalanceLog(fmt.Sprintf("Account %s, address: %s has an ending balance of %f in shard %d after the test", validatorAccount.Name, validatorAccount.Address, accountEndingBalance, testCase.StakingParameters.FromShardID), testCase.Verbose)
	}

	successfulValidatorCreation := tx.Success && accountEndingBalance.LT(expectedAccountEndingBalance) && validatorExists

	if successfulValidatorCreation {
		delegationTx, delegationSucceeded, err := staking.BasicDelegation(testCase, &delegatorAccount, &validatorAccount, nil)
		if err != nil {
			msg := fmt.Sprintf("Failed to delegate from account %s, address %s to validator %s, address: %s", delegatorAccount.Name, delegatorAccount.Address, validatorAccount.Name, validatorAccount.Address)
			testCase.HandleError(err, &validatorAccount, msg)
			return
		}
		testCase.Transactions = append(testCase.Transactions, delegationTx)

		successfulDelegation := delegationTx.Success && delegationSucceeded

		if successfulDelegation {
			undelegationTx, undelegationSucceeded, err := staking.BasicUndelegation(testCase, &delegatorAccount, &validatorAccount, &senderAccount)
			if err != nil {
				msg := fmt.Sprintf("Failed to undelegate from account %s, address %s to validator %s, address: %s", delegatorAccount.Name, delegatorAccount.Address, validatorAccount.Name, validatorAccount.Address)
				testCase.HandleError(err, &validatorAccount, msg)
				return
			}
			testCase.Transactions = append(testCase.Transactions, undelegationTx)

			testCase.Result = undelegationTx.Success && undelegationSucceeded
		}
	}

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
