package transactions

import (
	"fmt"
	"time"

	"github.com/harmony-one/harmony-tf/accounts"
	"github.com/harmony-one/harmony-tf/balances"
	"github.com/harmony-one/harmony-tf/config"
	"github.com/harmony-one/harmony-tf/funding"
	"github.com/harmony-one/harmony-tf/logger"
	"github.com/harmony-one/harmony-tf/rpc"
	"github.com/harmony-one/harmony-tf/testing"
)

// SameAccountScenario - executes a test case where the sender and receiver address is the same
func SameAccountScenario(testCase *testing.TestCase) {
	testing.Title(testCase, "header", testCase.Verbose)
	testCase.Executed = true
	testCase.StartedAt = time.Now().UTC()

	if testCase.ErrorOccurred(nil) {
		return
	}

	_, requiredFunding, err := funding.CalculateFundingDetails(testCase.Parameters.Amount, testCase.Parameters.ReceiverCount, testCase.Parameters.FromShardID)
	if testCase.ErrorOccurred(err) {
		return
	}

	accountName := accounts.GenerateTestCaseAccountName(testCase.Name, "Account")
	logger.AccountLog(fmt.Sprintf("Generating a new account: %s", accountName), testCase.Verbose)
	account, err := accounts.GenerateAccount(accountName)
	if testCase.ErrorOccurred(err) {
		return
	}

	logger.FundingLog(fmt.Sprintf("Funding account: %s, address: %s", account.Name, account.Address), testCase.Verbose)
	funding.PerformFundingTransaction(
		&config.Configuration.Funding.Account,
		testCase.Parameters.FromShardID,
		account.Address,
		testCase.Parameters.FromShardID,
		requiredFunding,
		-1,
		config.Configuration.Funding.Gas.Limit,
		config.Configuration.Funding.Gas.Price,
		config.Configuration.Funding.Timeout,
		config.Configuration.Funding.Retry.Attempts,
	)

	senderStartingBalance, err := balances.GetShardBalance(account.Address, testCase.Parameters.FromShardID)
	if testCase.ErrorOccurred(err) {
		return
	}

	receiverStartingBalance, err := balances.GetShardBalance(account.Address, testCase.Parameters.ToShardID)
	if testCase.ErrorOccurred(err) {
		return
	}

	logger.BalanceLog(fmt.Sprintf("Account %s (address: %s) has a starting balance of %f in source shard %d before the test", account.Name, account.Address, senderStartingBalance, testCase.Parameters.FromShardID), testCase.Verbose)

	if testCase.Parameters.FromShardID != testCase.Parameters.ToShardID {
		logger.BalanceLog(fmt.Sprintf("Account %s (address: %s) has a starting balance of %f in receiver shard %d before the test", account.Name, account.Address, receiverStartingBalance, testCase.Parameters.ToShardID), testCase.Verbose)
	}

	testCaseTx := rpc.SendTransaction(testCase, &account, &account)
	if testCase.ErrorOccurred(testCaseTx.Error) {
		return
	}

	testCase.Transactions = append(testCase.Transactions, testCaseTx)
	txResultColoring := logger.ResultColoring(testCaseTx.Success, true)

	logger.TransactionLog(fmt.Sprintf("Sent %f token(s) from %s (shard %d) to %s (shard %d) - transaction hash: %s, tx successful: %s", testCase.Parameters.Amount, account.Address, testCase.Parameters.FromShardID, account.Address, testCase.Parameters.ToShardID, testCaseTx.TransactionHash, txResultColoring), testCase.Verbose)

	/*if testCaseTx.Success && testCase.Parameters.FromShardID != testCase.Parameters.ToShardID {
		logger.BalanceLog(fmt.Sprintf("Because this is a cross shard transaction we need to wait an extra %d seconds to correctly receive the ending balance of the receiver account %s in shard %d", config.Configuration.Network.CrossShardTxWaitTime, account.Address, testCase.Parameters.ToShardID), testCase.Verbose)
		time.Sleep(time.Duration(config.Configuration.Network.CrossShardTxWaitTime) * time.Second)
	}*/

	receiverEndingBalance, err := balances.GetNonZeroShardBalance(account.Address, testCase.Parameters.ToShardID)
	if testCase.ErrorOccurred(err) {
		return
	}
	expectedReceiverEndingBalance := receiverStartingBalance.Add(testCase.Parameters.Amount)
	logger.BalanceLog(fmt.Sprintf("Account %s (address: %s) has an ending balance of %f in shard %d after the test - expected balance is %f", account.Name, account.Address, receiverEndingBalance, testCase.Parameters.ToShardID, expectedReceiverEndingBalance), testCase.Verbose)

	if testCase.Parameters.FromShardID == testCase.Parameters.ToShardID {
		// We should end up with a lesser amount when performing same shard transfers compared to the initial amount since we pay a gas fee
		testCase.Result = testCaseTx.Success && receiverEndingBalance.LTE(expectedReceiverEndingBalance)
	} else {
		// We should end up with an equal amount to starting balance + sent amount when performing cross shard shard transfers since the gas is deducted from the sender shard
		testCase.Result = testCaseTx.Success && receiverEndingBalance.Equal(expectedReceiverEndingBalance)
	}

	logger.TeardownLog(fmt.Sprintf("Performing test teardown (returning funds and removing account %s)\n", account.Name), testCase.Verbose)

	logger.ResultLog(testCase.Result, testCase.Expected, testCase.Verbose)
	testing.Title(testCase, "footer", testCase.Verbose)

	testing.Teardown(&account, testCase.Parameters.ToShardID, config.Configuration.Funding.Account.Address, testCase.Parameters.FromShardID)

	testCase.FinishedAt = time.Now().UTC()
}
