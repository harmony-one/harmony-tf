package transactions

import (
	"fmt"
	"sync"
	"time"

	"github.com/harmony-one/harmony-tf/accounts"
	"github.com/harmony-one/harmony-tf/balances"
	"github.com/harmony-one/harmony-tf/config"
	"github.com/harmony-one/harmony-tf/funding"
	"github.com/harmony-one/harmony-tf/logger"
	"github.com/harmony-one/harmony-tf/rpc"
	"github.com/harmony-one/harmony-tf/testing"
	"github.com/harmony-one/harmony-tf/transactions"
	"github.com/harmony-one/harmony/numeric"

	sdkAccounts "github.com/harmony-one/go-lib/accounts"
	sdkTxs "github.com/harmony-one/go-lib/transactions"
)

// Due to rate-limiting by RPC/explorer endpoints - don't run concurrently for now

// MultipleSenderScenario - runs a tests where multiple sender wallets are used to send to one respective new wallet
func MultipleSenderScenario(testCase *testing.TestCase) {
	testing.Title(testCase, "header", testCase.Verbose)

	if !config.Configuration.Framework.CanExecuteMemoryIntensiveTestCase() {
		testCase.ReportMemoryDismissal()
		return
	}

	testCase.Executed = true
	testCase.StartedAt = time.Now().UTC()

	if testCase.ErrorOccurred(nil) {
		return
	}

	_, _, err := funding.CalculateFundingDetails(testCase.Parameters.Amount, testCase.Parameters.SenderCount, testCase.Parameters.FromShardID)
	if testCase.ErrorOccurred(err) {
		return
	}

	receiverAccountName := accounts.GenerateTestCaseAccountName(testCase.Name, "Receiver")
	receiverAccount, err := accounts.GenerateAccount(receiverAccountName)
	if err != nil {
		msg := fmt.Sprintf("Failed to generate account %s", receiverAccountName)
		testCase.HandleError(err, &receiverAccount, msg)
		return
	}

	receiverStartingBalance, err := balances.GetShardBalance(receiverAccount.Address, testCase.Parameters.ToShardID)
	if err != nil {
		msg := fmt.Sprintf("Failed to retrieve balance for account %s, address: %s", receiverAccount.Name, receiverAccount.Address)
		testCase.HandleError(err, &receiverAccount, msg)
		return
	}

	logger.BalanceLog(fmt.Sprintf("Receiver account %s (address: %s) has a starting balance of %f in shard %d before the test", receiverAccount.Name, receiverAccount.Address, receiverStartingBalance, testCase.Parameters.ToShardID), testCase.Verbose)

	nameTemplate := accounts.GenerateTestCaseAccountName(testCase.Name, "Sender_")
	senderAccounts, err := funding.GenerateAndFundAccounts(testCase.Parameters.SenderCount, nameTemplate, testCase.Parameters.Amount, testCase.Parameters.FromShardID, testCase.Parameters.FromShardID)
	if err != nil {
		msg := fmt.Sprintf("Failed to generate a total of %d sender accounts", testCase.Parameters.SenderCount)
		testCase.HandleError(err, nil, msg)
		return
	}

	executeMultiSenderTransactions(testCase, senderAccounts, receiverAccount)
	txsSuccessful := (testCase.SuccessfulTxCount == testCase.Parameters.SenderCount)

	logger.TransactionLog(fmt.Sprintf("A total of %d/%d transactions were successful", testCase.SuccessfulTxCount, testCase.Parameters.SenderCount), testCase.Verbose)

	if txsSuccessful {
		receiverEndingBalance, err := balances.GetNonZeroShardBalance(receiverAccount.Address, testCase.Parameters.ToShardID)
		if testCase.ErrorOccurred(err) {
			return
		}
		decSenderCount := numeric.NewDec(testCase.Parameters.SenderCount)
		var expectedBalance numeric.Dec

		if testCase.Expected {
			expectedBalance = testCase.Parameters.Amount.Mul(decSenderCount)
		} else {
			expectedBalance = numeric.NewDec(0)
		}

		testCase.Result = (txsSuccessful && receiverEndingBalance.Equal(expectedBalance))

		logger.BalanceLog(fmt.Sprintf("Receiver account %s (address: %s) has an ending balance of %f in shard %d after the test - expected balance: %f", receiverAccount.Name, receiverAccount.Address, receiverEndingBalance, testCase.Parameters.ToShardID, expectedBalance), testCase.Verbose)
	} else {
		testCase.Result = false
	}

	logger.TeardownLog("Performing test teardown (returning funds and removing accounts)", testCase.Verbose)
	logger.ResultLog(testCase.Result, testCase.Expected, testCase.Verbose)

	multipleSendersTeardown(testCase, senderAccounts, receiverAccount)
	testing.Title(testCase, "footer", testCase.Verbose)

	testCase.FinishedAt = time.Now().UTC()
}

func executeMultiSenderTransactions(testCase *testing.TestCase, senderAccounts []sdkAccounts.Account, receiverAccount sdkAccounts.Account) {
	if testCase.Parameters.RPCPrefix == "eth" {
		ethChainID := rpc.GenerateEthereumChainID(config.Configuration.Network.Name, testCase.Parameters.FromShardID)
		config.Configuration.Network.ChangeRPCSettings(testCase.Parameters.RPCPrefix, ethChainID)
	}

	txs := make(chan sdkTxs.Transaction, testCase.Parameters.SenderCount)
	var waitGroup sync.WaitGroup

	for _, senderAccount := range senderAccounts {
		waitGroup.Add(1)
		go executeSenderTransaction(testCase, senderAccount, receiverAccount, txs, &waitGroup)
	}

	waitGroup.Wait()
	close(txs)

	testCase.SuccessfulTxCount = 0
	for tx := range txs {
		testCase.Transactions = append(testCase.Transactions, tx)
		if tx.Success {
			testCase.SuccessfulTxCount++
		}
	}

	config.Configuration.Network.RevertRPCSettings()
}

func executeSenderTransaction(testCase *testing.TestCase, senderAccount sdkAccounts.Account, receiverAccount sdkAccounts.Account, responses chan<- sdkTxs.Transaction, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()
	var testCaseTx sdkTxs.Transaction
	var rawTx map[string]interface{}
	var err error
	balanceRetrieved := true

	senderStartingBalance, err := balances.GetNonZeroShardBalance(senderAccount.Address, testCase.Parameters.FromShardID)
	if err != nil {
		balanceRetrieved = false
	}

	if !senderStartingBalance.IsNil() && !senderStartingBalance.IsZero() {
		txData := testCase.Parameters.GenerateTxData()
		logger.BalanceLog(fmt.Sprintf("Sender account %s (address: %s) has a starting balance of %f in shard %d before the test", senderAccount.Name, senderAccount.Address, senderStartingBalance, testCase.Parameters.FromShardID), testCase.Verbose)
		logger.TransactionLog(fmt.Sprintf("Sending transaction of %f token(s) from %s (shard %d) to %s (shard %d), tx data size: %d byte(s)", testCase.Parameters.Amount, senderAccount.Address, testCase.Parameters.FromShardID, receiverAccount.Address, testCase.Parameters.ToShardID, len(txData)), testCase.Verbose)
		logger.TransactionLog(fmt.Sprintf("Will wait up to %d seconds to let the transaction get finalized", testCase.Parameters.Timeout), testCase.Verbose)

		if testCase.Parameters.RPCPrefix == "eth" {
			rawTx, err = transactions.SendEthTransaction(&senderAccount, testCase.Parameters.FromShardID, receiverAccount.Address, testCase.Parameters.Amount, testCase.Parameters.Nonce, testCase.Parameters.Gas.Limit, testCase.Parameters.Gas.Price, txData, testCase.Parameters.Timeout)
		} else {
			rawTx, err = transactions.SendTransaction(&senderAccount, testCase.Parameters.FromShardID, receiverAccount.Address, testCase.Parameters.ToShardID, testCase.Parameters.Amount, testCase.Parameters.Nonce, testCase.Parameters.Gas.Limit, testCase.Parameters.Gas.Price, txData, testCase.Parameters.Timeout)
		}

		testCaseTx = sdkTxs.ToTransaction(senderAccount.Address, testCase.Parameters.FromShardID, receiverAccount.Address, testCase.Parameters.ToShardID, rawTx, err)

		if testCaseTx.Error != nil {
			logger.ErrorLog(fmt.Sprintf("Failed to send %f coins from %s (shard %d) to %s (shard %d) - error: %s", testCase.Parameters.Amount, senderAccount.Address, testCase.Parameters.FromShardID, receiverAccount.Address, testCase.Parameters.ToShardID, testCaseTx.Error.Error()), testCase.Verbose)
		} else {
			txResultColoring := logger.ResultColoring(testCaseTx.Success, true)
			logger.TransactionLog(fmt.Sprintf("Sent %f coins from %s (shard %d) to %s (shard %d) - transaction hash: %s, tx successful: %s", testCase.Parameters.Amount, senderAccount.Address, testCase.Parameters.FromShardID, receiverAccount.Address, testCase.Parameters.ToShardID, testCaseTx.TransactionHash, txResultColoring), testCase.Verbose)
		}
	} else {
		balanceRetrieved = false
	}

	if !balanceRetrieved {
		logger.FundingLog(fmt.Sprintf("Couldn't proceed with executing transaction since sender account %s hasn't been funded properly, balance is: %f", senderAccount.Address, senderStartingBalance), testCase.Verbose)

		testCaseTx = sdkTxs.Transaction{
			Success: false,
			Error:   fmt.Errorf("sender account %s wasn't funded properly, balance is: %f", senderAccount.Address, senderStartingBalance),
		}
	}

	responses <- testCaseTx
}

func multipleSendersTeardown(testCase *testing.TestCase, senderAccounts []sdkAccounts.Account, receiverAccount sdkAccounts.Account) {
	var waitGroup sync.WaitGroup
	waitGroup.Add(1 + len(senderAccounts))

	for _, senderAccount := range senderAccounts {
		go testing.AsyncTeardown(&senderAccount, testCase.Parameters.FromShardID, config.Configuration.Funding.Account.Address, testCase.Parameters.FromShardID, &waitGroup)
	}
	go testing.AsyncTeardown(&receiverAccount, testCase.Parameters.ToShardID, config.Configuration.Funding.Account.Address, testCase.Parameters.FromShardID, &waitGroup)

	waitGroup.Wait()
}
