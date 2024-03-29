package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/csv"
	"fmt"
	"log"
	"math"
	"math/big"
	"modules/licensing"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	pcsRouter "tgbot/contracts"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Config struct {
	Parameters struct {
		Host       string  `yaml:"host"`
		Contracts  string  `yaml:"contracts"`
		Bought     string  `yaml:"bought"`
		PCSAddress string  `yaml:"pcsaddress"`
		BNBAddress string  `yaml:"bnbaddress"`
		PrivateKey string  `yaml:"privatekey"`
		AmountIn   float64 `yaml:"amountin"`
		GasLimit   uint64  `yaml:"gaslimit"`
		GasPrice   int64   `yaml:"gasprice"`
		TGBotApi   string  `yaml:"tgbotapi"`
	} `yaml:"parameters"`
}

func main() {
	licensing.CheckTelegramBotLicense("http://wisdom-bots.com:3002", false, false)
	fmt.Println("Starting...")

	f, err := os.Open("./config.yml")
	if err != nil {
		fmt.Println("Unable to open config. Make sure it is present.")
		log.Fatal(err)
	}
	defer f.Close()

	var cfg Config
	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		fmt.Println("Error reading config. Please make sure it is formatted correctly.")
		log.Fatal(err)
	}

	bot, err := tgbotapi.NewBotAPI(cfg.Parameters.TGBotApi)
	if err != nil {
		fmt.Println("Unable to connect to Telegram bot.")
		log.Fatal(err)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	filePath := cfg.Parameters.Contracts
	updates := bot.GetUpdatesChan(u)
	fmt.Println("Listening for updates now!")
	for update := range updates {
		f, _ := os.OpenFile(filePath, os.O_RDWR, 0777)
		writer := csv.NewWriter(f)
		contract := getContract(update)
		if !exists(contract, f) {
			writer.Write([]string{contract})
			writer.Flush()
			f.Close()
			buy(contract, cfg)
		}
	}

}

func exists(contract string, f *os.File) bool {
	reader := csv.NewReader(f)
	contracts, _ := reader.ReadAll()
	if len(contracts) != 0 {
		for _, existingContract := range contracts {
			if runtime.GOOS == "windows" {
				existingContract[0] = strings.TrimRight(existingContract[0], "\r\n")
			} else {
				existingContract[0] = strings.TrimRight(existingContract[0], "\n")
			}
			if contract == existingContract[0] {
				return true
			}
		}
		return false
	} else {
		return true
	}
}

func getContract(update tgbotapi.Update) string {
	regex := regexp.MustCompile(`0x.*`)
	contractExt := regex.FindAllString(update.Message.Text, -1)[0]
	runes := []rune(contractExt)
	contract := string(runes[0:42])
	return contract
}

func buy(contract string, cfg Config) {
	client, err := ethclient.Dial(cfg.Parameters.Host)
	if err != nil {
		fmt.Println("Error connecting to node.")
		log.Fatal(err)
	}

	contractAddress := common.HexToAddress(contract)
	pcsRouterAddress := common.HexToAddress(cfg.Parameters.PCSAddress)
	bnbAddress := common.HexToAddress(cfg.Parameters.BNBAddress)
	privateKey, err := crypto.HexToECDSA(cfg.Parameters.PrivateKey)
	if err != nil {
		fmt.Println("Private key error. Please make sure it is entered correctly")
		log.Fatal(err)
	}

	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		fmt.Println("Unable to obtain ChainID.")
		log.Fatal(err)
	}
	auth, _ := bind.NewKeyedTransactorWithChainID(privateKey, (*big.Int)(chainID))
	if err != nil {
		log.Fatal(err)
	}
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("cannot assert type: publicKey is not of type *ecdsa.PublicKey")
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		fmt.Println("Unable to retrieve pending nonce.")
		log.Fatal(err)
	}
	txNonce := big.NewInt(int64(nonce))
	amountIn := big.NewInt(int64(cfg.Parameters.AmountIn * (math.Pow(10.0, 18.0))))
	auth.GasLimit = cfg.Parameters.GasLimit // in units
	auth.GasPrice = big.NewInt(cfg.Parameters.GasPrice * (1000000000))
	instance, err := pcsRouter.NewPcsRouter(pcsRouterAddress, client)
	if err != nil {
		log.Fatal(err)
	}
	opts := &bind.CallOpts{
		Pending: false,
		Context: context.Background(),
	}
	path := []common.Address{bnbAddress, contractAddress}
	out, err := instance.GetAmountsOut(opts, amountIn, path)
	if err != nil {
		fmt.Println("Unable to determine amount out.")
		log.Fatal(err)
	}
	minAmtOut := out[1].Mul(out[1], big.NewInt(85)).Div(out[1], big.NewInt(100))
	transactOps := &bind.TransactOpts{
		From:     fromAddress,
		Nonce:    txNonce,
		Signer:   auth.Signer,
		Value:    amountIn,
		GasPrice: auth.GasPrice,
		GasLimit: auth.GasLimit,
		Context:  context.Background(),
	}
	deadline := big.NewInt(time.Now().Unix() + 1200)
	tx, err := instance.SwapExactETHForTokensSupportingFeeOnTransferTokens(transactOps, minAmtOut, path, fromAddress, deadline)
	if err != nil {
		fmt.Printf("Transaction failed.\n%q", err)
	}
	if err == nil {
		fmt.Printf("Transaction receipt: %v", tx.Hash())
		f, _ := os.OpenFile(cfg.Parameters.Bought, os.O_RDWR, 0777)
		writer := csv.NewWriter(f)
		writer.Write([]string{fmt.Sprintf("Transaction receipt: %v", tx.Hash())})
		writer.Flush()
		f.Close()
	}

}
