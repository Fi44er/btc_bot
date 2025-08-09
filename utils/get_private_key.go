package utils

import (
	"fmt"
	"log"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
)

func GetAddressPrivateKey(masterKeyStr string, targetAddress string, params *chaincfg.Params) (string, error) {
	// params := &chaincfg.TestNet3Params
	masterKey, err := hdkeychain.NewKeyFromString(masterKeyStr)
	if err != nil {
		log.Fatalf("Ошибка декодирования мастер-ключа: %v", err)
	}

	for i := uint32(0); i < 10000; i++ {
		childKey, err := masterKey.Derive(i)
		if err != nil {
			log.Printf("Ошибка получения дочернего ключа для индекса %d: %v", i, err)
			continue
		}

		address, err := childKey.Address(params)
		if err != nil {
			log.Printf("Ошибка генерации адреса для индекса %d: %v", i, err)
			continue
		}

		if address.String() == targetAddress {
			privKey, err := childKey.ECPrivKey()
			if err != nil {
				log.Printf("Ошибка получения приватного ключа: %v", err)
			}

			wif, err := btcutil.NewWIF(privKey, params, true)
			if err != nil {
				log.Printf("Ошибка создания WIF: %v", err)
			}

			fmt.Printf("Найден адрес!\nИндекс: %d\nПриватный ключ (WIF): %s\n", i, wif.String())
			return wif.String(), nil
		}
	}

	return "", fmt.Errorf("адрес не найден")
}
