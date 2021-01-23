package keys

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/crypto"
	sdkAccounts "github.com/harmony-one/go-lib/accounts"
	goSdkAccount "github.com/harmony-one/go-sdk/pkg/account"
	"github.com/harmony-one/go-sdk/pkg/address"
	goSdkAddress "github.com/harmony-one/go-sdk/pkg/address"
	"github.com/harmony-one/harmony-tf/accounts"
	"github.com/harmony-one/harmony-tf/balances"
	"github.com/harmony-one/harmony-tf/config"
	"github.com/harmony-one/harmony-tf/utils"
)

var (
	// KeyMapping is a map where the key is the address of an identified keystore key and the value is the path to said keystore key
	KeyMapping map[string]string
)

// LoadKeys - loads all keys using the private keys file + keystore files
func LoadKeys() (allAccounts []sdkAccounts.Account, err error) {
	privKeyAccounts, err := LoadPrivateKeys()
	if err != nil {
		return nil, err
	}

	keystoreAccounts, err := LoadKeystoreKeys()
	if err != nil {
		return nil, err
	}

	allAccounts = append(privKeyAccounts, keystoreAccounts...)

	return allAccounts, nil
}

// LoadPrivateKeys - loads the source accounts using a .txt file including new line separated private keys
func LoadPrivateKeys() (accs []sdkAccounts.Account, err error) {
	unfilteredAccounts := []sdkAccounts.Account{}

	path := filepath.Join(config.Configuration.Framework.BasePath, "keys", config.Configuration.Network.Name, "private_keys.txt")
	privateKeys, err := utils.FileToLines(path)
	if err != nil {
		return nil, err
	}

	if len(privateKeys) > 0 {
		fmt.Println(fmt.Sprintf("Found a total of %d private key(s) to import to the keystore", len(privateKeys)))
		for _, privateKey := range privateKeys {
			address, err := PrivateKeyToAddress(privateKey)

			if err == nil {
				keyName := utils.PrefixAddress(config.Configuration.Network.Name, address)
				account, err := accounts.ImportPrivateKeyAccount(privateKey, keyName, address)

				if account.Address != "" && err == nil {
					unfilteredAccounts = append(unfilteredAccounts, account)
				}
			}
		}

		accs, err = FilterKeys(unfilteredAccounts)
		if err != nil {
			return nil, err
		}
	}

	return accs, nil
}

// LoadKeystoreKeys - loads the source accounts using keystore files and imports them into the keystore
func LoadKeystoreKeys() (accs []sdkAccounts.Account, err error) {
	KeyMapping = make(map[string]string)
	unfilteredAccounts := []sdkAccounts.Account{}

	path := filepath.Join(config.Configuration.Framework.BasePath, "keys", config.Configuration.Network.Name)
	if err := IdentifyKeystoreKeys(path); err != nil {
		return nil, err
	}

	fmt.Println(fmt.Sprintf("Found a total of %d keystore file(s) in %s to import to the keystore", len(KeyMapping), path))

	index := 0
	for address, path := range KeyMapping {
		keyName := utils.PrefixAddress(config.Configuration.Network.Name, address)

		account, err := accounts.ImportKeystoreAccount(path, keyName, address)
		if account.Address != "" && err == nil {
			unfilteredAccounts = append(unfilteredAccounts, account)
		}
		index++
	}

	accs, err = FilterKeys(unfilteredAccounts)
	if err != nil {
		return nil, err
	}

	return accs, nil
}

// FilterKeys - filters keys based on available balance and potentially removes keys without any balances (if enabled in the configuration)
func FilterKeys(unfilteredAccounts []sdkAccounts.Account) (accounts []sdkAccounts.Account, err error) {
	hasFunds, missingFunds, err := balances.FilterMinimumBalanceAccounts(unfilteredAccounts, config.Configuration.Funding.MinimumFunds)
	if err != nil {
		return nil, err
	}

	if config.Configuration.Account.RemoveEmpty {
		for _, account := range missingFunds {
			keyName := utils.PrefixAddress(config.Configuration.Network.Name, account.Address)
			//fmt.Println(fmt.Sprintf("Account %s, address: %s doesn't hold any funds on the %s - removing the account...", keyName, address, strings.Title(config.Configuration.Network.Name)))
			goSdkAccount.RemoveAccount(keyName)

			if len(KeyMapping) > 0 {
				if sourcePath, ok := KeyMapping[account.Address]; ok {
					os.RemoveAll(sourcePath)
				}
			}
		}
	}

	for _, account := range hasFunds {
		accounts = append(accounts, account)
	}

	return accounts, nil
}

// IdentifyKeystoreKeys - identifies the key store files in a given path - also supports an unlimited amount of subdirectories
func IdentifyKeystoreKeys(path string) error {
	var files []string
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		files = append(files, path)
		return nil
	})

	if err != nil {
		return err
	}

	for _, file := range files {
		if filepath.Ext(file) != ".txt" {
			filePath, err := filepath.Abs(file)

			if err != nil {
				return err
			}

			keyData, err := utils.ReadFileToString(filePath)

			if err == nil {
				keyDetails, err := parseKeystoreJSON(keyData)

				if err == nil {
					if address, ok := keyDetails["address"]; ok {
						if address.(string) != "" {
							KeyMapping[address.(string)] = filePath
						}
					}
				}
			}
		}
	}

	return nil
}

func parseKeystoreJSON(data string) (map[string]interface{}, error) {
	var rawData interface{}
	err := json.Unmarshal([]byte(data), &rawData)

	if err != nil {
		return nil, err
	}

	jsonData := rawData.(map[string]interface{})
	ethAddress := jsonData["address"].(string)
	bech32Address := address.ToBech32(address.Parse(ethAddress))

	if bech32Address != "" {
		jsonData["address"] = bech32Address
	}

	return jsonData, nil
}

// PrivateKeyToAddress - converts a given private key to a Bech32 formatted representation of its public key
func PrivateKeyToAddress(privateKey string) (string, error) {
	key, err := crypto.HexToECDSA(privateKey)

	if err != nil {
		return "", err
	}

	address := crypto.PubkeyToAddress(key.PublicKey)
	formattedAddress := goSdkAddress.ToBech32(address)

	return formattedAddress, nil
}
