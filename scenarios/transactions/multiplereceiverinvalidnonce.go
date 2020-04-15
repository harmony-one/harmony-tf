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
	"github.com/harmony-one/harmony-tf/testing"
	"github.com/harmony-one/harmony-tf/transactions"

	sdkAccounts "github.com/harmony-one/go-lib/accounts"
	sdkNetworkNonce "github.com/harmony-one/go-lib/network/rpc/nonces"
	sdkTxs "github.com/harmony-one/go-lib/transactions"
)

// Due to rate-limiting by RPC/explorer endpoints - don't run concurrently for now

// MultipleReceiverInvalidNonceScenario - runs a tests where multiple receiver wallets receive txs with the exact same nonce
func MultipleReceiverInvalidNonceScenario(testCase *testing.TestCase) {
	testing.Title(testCase, "header", testCase.Verbose)

	if !config.Configuration.Framework.CanExecuteMemoryIntensiveTestCase() {
		testCase.ReportMemoryDismissal()
		return
	}

	testCase.Executed = true
	if testCase.ReportError() {
		return
	}

	testCase.StartedAt = time.Now().UTC()

	fundingAccountBalance := testing.RetrieveFundingAccountBalanceOrError(testCase)
	if testCase.Error != nil {
		return
	}

	fundingAmount, err := funding.CalculateFundingAmount(testCase.Parameters.Amount, fundingAccountBalance, testCase.Parameters.SenderCount)
	if err != nil {
		testCase.SetError(err)
		return
	}

	senderAccountName := accounts.GenerateTestCaseAccountName(testCase.Name, "Sender")
	logger.AccountLog(fmt.Sprintf("Generating a new sender account: %s", senderAccountName), testCase.Verbose)
	senderAccount, err := accounts.GenerateAccount(senderAccountName)
	if err != nil {
		testing.HandleError(testCase, &senderAccount, fmt.Sprintf("Failed to generate account %s", senderAccountName), err)
		return
	}

	logger.FundingLog(fmt.Sprintf("Funding sender account: %s, address: %s", senderAccount.Name, senderAccount.Address), testCase.Verbose)
	funding.PerformFundingTransaction(
		&config.Configuration.Funding.Account,
		testCase.Parameters.FromShardID,
		senderAccount.Address,
		testCase.Parameters.ToShardID,
		fundingAmount, -1,
		config.Configuration.Funding.Gas.Limit,
		config.Configuration.Funding.Gas.Price,
		config.Configuration.Funding.Timeout,
		config.Configuration.Funding.Retry.Attempts,
	)

	nameTemplate := accounts.GenerateTestCaseAccountName(testCase.Name, "Receiver_")
	receiverAccounts := accounts.AsyncGenerateMultipleAccounts(nameTemplate, testCase.Parameters.ReceiverCount)

	executeMultiInvalidNonceTransactions(testCase, senderAccount, receiverAccounts)

	logger.TransactionLog(fmt.Sprintf("A total of %d/%d transactions were successful", testCase.SuccessfulTxCount, testCase.Parameters.ReceiverCount), testCase.Verbose)

	testCase.Result = (testCase.SuccessfulTxCount == testCase.Parameters.ReceiverCount)

	logger.TeardownLog("Performing test teardown (returning funds and removing accounts)\n", testCase.Verbose)
	logger.ResultLog(testCase.Result, testCase.Expected, testCase.Verbose)

	multipleReceiversTeardown(testCase, senderAccount, receiverAccounts)
	testing.Title(testCase, "footer", testCase.Verbose)

	testCase.FinishedAt = time.Now().UTC()
}

func executeMultiInvalidNonceTransactions(testCase *testing.TestCase, senderAccount sdkAccounts.Account, receiverAccounts []sdkAccounts.Account) {
	rpcClient, _ := config.Configuration.Network.API.RPCClient(testCase.Parameters.FromShardID)
	nonce := -1
	receivedNonce := sdkNetworkNonce.CurrentNonce(rpcClient, senderAccount.Address)
	nonce = int(receivedNonce)

	logger.TransactionLog(fmt.Sprintf("Current nonce for sender account: %s, address: %s is %d", senderAccount.Name, senderAccount.Address, nonce), testCase.Verbose)

	txs := make(chan sdkTxs.Transaction, testCase.Parameters.ReceiverCount)
	var waitGroup sync.WaitGroup

	for _, receiverAccount := range receiverAccounts {
		waitGroup.Add(1)
		go executeInvalidNonceTransaction(testCase, senderAccount, receiverAccount, nonce, txs, &waitGroup)
	}

	waitGroup.Wait()
	close(txs)

	testCase.SuccessfulTxCount = int64(0)
	for tx := range txs {
		testCase.Transactions = append(testCase.Transactions, tx)
		if tx.Success {
			testCase.SuccessfulTxCount++
		}
	}
}

func executeInvalidNonceTransaction(testCase *testing.TestCase, senderAccount sdkAccounts.Account, receiverAccount sdkAccounts.Account, nonce int, responses chan<- sdkTxs.Transaction, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()
	var testCaseTx sdkTxs.Transaction
	senderStartingBalance, _ := balances.GetShardBalance(senderAccount.Address, testCase.Parameters.FromShardID)

	if testCase.Parameters.FromShardID != testCase.Parameters.ToShardID && (senderStartingBalance.IsNil() || senderStartingBalance.IsZero()) {
		logger.FundingLog(fmt.Sprintf("We need to wait an extra %d seconds for the funding to arrive to sender account %s in shard %d", config.Configuration.Network.CrossShardTxWaitTime, senderAccount.Address, testCase.Parameters.FromShardID), testCase.Verbose)
		time.Sleep(time.Duration(config.Configuration.Network.CrossShardTxWaitTime) * time.Second)
		senderStartingBalance, _ = balances.GetShardBalance(senderAccount.Address, testCase.Parameters.FromShardID)
	}

	if !senderStartingBalance.IsZero() {
		logger.AccountLog(fmt.Sprintf("Generated a new receiver account: %s, address: %s", receiverAccount.Name, receiverAccount.Address), testCase.Verbose)
		logger.AccountLog(fmt.Sprintf("Using sender account %s (address: %s) and receiver account %s (address : %s)", senderAccount.Name, senderAccount.Address, receiverAccount.Name, receiverAccount.Address), testCase.Verbose)
		logger.BalanceLog(fmt.Sprintf("Sender account %s (address: %s) has a starting balance of %f in shard %d before the test", senderAccount.Name, senderAccount.Address, senderStartingBalance, testCase.Parameters.FromShardID), testCase.Verbose)
		logger.BalanceLog(fmt.Sprintf("Will wait up to %d seconds to let the transaction get finalized", testCase.Parameters.Timeout), testCase.Verbose)

		txData := testCase.Parameters.GenerateTxData()
		logger.TransactionLog(fmt.Sprintf("Sending transaction of %f token(s) from %s (shard %d) to %s (shard %d), tx data size: %d byte(s)", testCase.Parameters.Amount, senderAccount.Address, testCase.Parameters.FromShardID, receiverAccount.Address, testCase.Parameters.ToShardID, len(txData)), testCase.Verbose)

		rawTx, err := transactions.SendTransaction(&senderAccount, testCase.Parameters.FromShardID, receiverAccount.Address, testCase.Parameters.ToShardID, testCase.Parameters.Amount, nonce, testCase.Parameters.Gas.Limit, testCase.Parameters.Gas.Price, txData, testCase.Parameters.Timeout)
		testCaseTx = sdkTxs.ToTransaction(senderAccount.Address, testCase.Parameters.FromShardID, receiverAccount.Address, testCase.Parameters.ToShardID, rawTx, err)
		txResultColoring := logger.ResultColoring(testCaseTx.Success, true).Render(fmt.Sprintf("tx successful: %t", testCaseTx.Success))

		logger.TransactionLog(fmt.Sprintf("Sent %f token(s) from %s to %s - transaction hash: %s, %s", testCase.Parameters.Amount, config.Configuration.Funding.Account.Address, receiverAccount.Address, testCaseTx.TransactionHash, txResultColoring), testCase.Verbose)
	} else {
		logger.FundingLog(fmt.Sprintf("Couldn't proceed with executing transaction since sender account %s hasn't been funded properly, balance is: %f", senderAccount.Address, senderStartingBalance), testCase.Verbose)

		testCaseTx = sdkTxs.Transaction{
			Success: false,
			Error:   fmt.Errorf("sender account %s wasn't funded properly, balance is: %f", senderAccount.Address, senderStartingBalance),
		}
	}

	responses <- testCaseTx
}

func multipleReceiversTeardown(testCase *testing.TestCase, senderAccount sdkAccounts.Account, receiverAccounts []sdkAccounts.Account) {
	var waitGroup sync.WaitGroup
	waitGroup.Add(1 + len(receiverAccounts))

	go testing.AsyncTeardown(&senderAccount, testCase.Parameters.FromShardID, config.Configuration.Funding.Account.Address, testCase.Parameters.ToShardID, &waitGroup)
	for _, receiverAccount := range receiverAccounts {
		go testing.AsyncTeardown(&receiverAccount, testCase.Parameters.ToShardID, config.Configuration.Funding.Account.Address, testCase.Parameters.FromShardID, &waitGroup)
	}

	waitGroup.Wait()
}
