package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"go-ido-bot/contract/erc20"
	"go-ido-bot/contract/factory"
	"go-ido-bot/contract/routerV2"
	"log"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

var ZERO_ADDRESS = common.HexToAddress("0x0000000000000000000000000000000000000000")

type Bot struct {
	client       *ethclient.Client
	privateKey   *ecdsa.PrivateKey
	userAddr     common.Address
	inTokenAddr  common.Address
	outTokenAddr common.Address
	routerAddr   common.Address
	factoryAddr  common.Address
}

func NewBot(client *ethclient.Client, privateKey *ecdsa.PrivateKey, inTokenAddr common.Address, outTokenAddr common.Address, routerAddr common.Address, factoryAddr common.Address) *Bot {
	bot := new(Bot)
	bot.client = client
	bot.privateKey = privateKey
	bot.inTokenAddr = inTokenAddr
	bot.outTokenAddr = outTokenAddr
	bot.routerAddr = routerAddr
	bot.factoryAddr = factoryAddr

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("cannot assert type: publicKey is not of type *ecdsa.PublicKey")
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	bot.userAddr = fromAddress
	return bot
}

func inputHandler() string {
	var input string
	fmt.Scanln(&input)
	input = strings.ToLower(input)
	return input
}

func (bot *Bot) getPairAddr() common.Address {
	factory, err := factory.NewFactory(bot.factoryAddr, bot.client)
	if err != nil {
		log.Fatal("cannot init router contract")
	}

	pairAddr, err := factory.GetPair(&bind.CallOpts{}, bot.inTokenAddr, bot.outTokenAddr)
	if err != nil {
		log.Fatal(err.Error())
	}

	return pairAddr
}

func (bot *Bot) getDecimal(tokenAddr common.Address) uint8 {
	token, err := erc20.NewErc20(tokenAddr, bot.client)
	if err != nil {
		log.Fatal("cannot init token contract")
	}

	decimal, err := token.Decimals(&bind.CallOpts{})
	if err != nil {
		log.Fatal("cannot get token decimal")
	}

	return decimal
}

func (bot *Bot) getBalanceOf(tokenAddr common.Address, walletAddr common.Address) *big.Int {
	token, err := erc20.NewErc20(tokenAddr, bot.client)
	if err != nil {
		log.Fatal("cannot init outToken contract")
	}

	balance, err := token.BalanceOf(&bind.CallOpts{}, walletAddr)
	if err != nil {
		log.Fatal("cannot init token contract")
	}

	return balance
}

func (bot *Bot) getTransOpt(gasPrice float64, gasLimit uint, nonce uint64) *bind.TransactOpts {

	gasPrice = gasPrice * 1000000000
	bigIntGasPrice := big.NewInt(int64(gasPrice))

	auth := bind.NewKeyedTransactor(bot.privateKey)
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)       // in wei
	auth.GasLimit = uint64(gasLimit) // in units
	auth.GasPrice = bigIntGasPrice
	return auth
}

func (bot *Bot) swap(to common.Address, amount *big.Int, deadline *big.Int, isETH bool, transOpt *bind.TransactOpts) *types.Transaction {
	router, err := routerV2.NewRouterV2(bot.routerAddr, bot.client)
	if err != nil {
		log.Fatal(err.Error())
	}

	path := []common.Address{bot.inTokenAddr, bot.outTokenAddr}

	var tx *types.Transaction
	if isETH {
		transOpt.Value = amount
		tx, err = router.SwapExactETHForTokens(transOpt, big.NewInt(0), path, to, deadline)
		if err != nil {
			log.Fatal(err.Error())
		}
	} else {
		tx, err = router.SwapExactTokensForTokens(transOpt, amount, big.NewInt(0), path, to, deadline)
		if err != nil {
			log.Fatal(err.Error())
		}
	}
	return tx
}

func (bot *Bot) getNonce() uint64 {
	nonce, err := bot.client.PendingNonceAt(context.Background(), bot.userAddr)
	if err != nil {
		log.Fatal(err)
	}

	return nonce
}

func (bot *Bot) getTxReceipt(tx *types.Transaction) *types.Receipt {
	receipt, err := bot.client.TransactionReceipt(context.Background(), tx.Hash())
	if err != nil {
		log.Fatal(err.Error())
	}
	return receipt
}

func main() {

	fmt.Print("RPC endpoint: ")
	rpc := inputHandler()

	client, err := ethclient.Dial(rpc)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("we have a connection")

	fmt.Print("Enter Your Private Key: ")
	privateKeyString := inputHandler()

	if strings.Contains(privateKeyString, "0x") {
		privateKeyString = privateKeyString[2:]
	}

	privateKey, err := crypto.HexToECDSA(privateKeyString)
	if err != nil {
		fmt.Println("invalid private key")
		log.Fatal(err)
	}

	fmt.Print("Router V2 Address: ")
	routerV2AddrString := inputHandler()
	routerAddr := common.HexToAddress(routerV2AddrString)

	fmt.Print("Factory Address: ")
	factoryAddrString := inputHandler()
	factoryAddr := common.HexToAddress(factoryAddrString)

	var isETH = false
	fmt.Print("is ETH?(Y/N): ")
	isETHString := inputHandler()
	if isETHString == "y" {
		isETH = true
	}

	var inTokenAddr common.Address
	fmt.Print("inToken Address: ")
	inTokenAddrString := inputHandler()
	inTokenAddr = common.HexToAddress(inTokenAddrString)

	fmt.Print("outToken Address: ")
	outTokenAddrString := inputHandler()
	outTokenAddr := common.HexToAddress(outTokenAddrString)

	fmt.Print("amount In: ")
	amountInString := inputHandler()
	amount, err := strconv.ParseFloat(amountInString, 64)
	if err != nil {
		log.Fatal("input amount error")
	}

	fmt.Print("gas price: ")
	gasPriceString := inputHandler()
	gasPrice, err := strconv.ParseFloat(gasPriceString, 64)
	if err != nil {
		log.Fatal("input amount error")
	}

	bot := NewBot(client, privateKey, inTokenAddr, outTokenAddr, routerAddr, factoryAddr)

	for {
		pairAddr := bot.getPairAddr()
		if pairAddr == ZERO_ADDRESS {
			fmt.Println(time.Now(), " pool not created yet!")
			continue
		} else {
			fmt.Println("pool address is: ", pairAddr.Hex())
			for {
				poolBalance := bot.getBalanceOf(bot.inTokenAddr, pairAddr)

				if poolBalance.Cmp(big.NewInt(0)) > 0 {
					nonce := bot.getNonce()

					var bigIntAmount *big.Int
					if isETH {
						amount *= 1000000000000000000
						bigIntAmount = big.NewInt(int64(amount))
					} else {
						decimal := bot.getDecimal(bot.inTokenAddr)
						amount *= float64(decimal)
						bigIntAmount = big.NewInt(int64(amount))
					}

					transOpt := bot.getTransOpt(gasPrice, 3000000, nonce)
					tx := bot.swap(bot.userAddr, bigIntAmount, big.NewInt(99999999999), isETH, transOpt)

					receipt := bot.getTxReceipt(tx)
					fmt.Println(receipt.Status)
					amountOut := bot.getBalanceOf(bot.outTokenAddr, bot.userAddr)
					bitFloatAmountOut := new(big.Float).SetInt(amountOut)
					outTokenDecimal := big.NewFloat(float64(bot.getDecimal(bot.outTokenAddr)))
					bigAmountOut := new(big.Float).Quo(bitFloatAmountOut, outTokenDecimal)

					fmt.Println("you get: ", bigAmountOut)
					return
				} else {
					fmt.Println(time.Now(), "pool is still empty")
				}
			}
		}

	}

}

func main2() {
	var inTE, outTE *walk.TextEdit

	MainWindow{
		Title:   "SCREAMO",
		MinSize: Size{600, 400},
		Layout:  VBox{},
		Children: []Widget{
			HSplitter{
				Children: []Widget{
					TextEdit{AssignTo: &inTE},
					TextEdit{AssignTo: &outTE, ReadOnly: true},
				},
			},
			PushButton{
				Text: "SCREAM",
				OnClicked: func() {
					outTE.SetText(strings.ToUpper(inTE.Text()))
				},
			},
		},
	}.Run()
}
