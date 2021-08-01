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

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/widget"
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

func (bot *Bot) getPairAddr() (common.Address, error) {
	factory, err := factory.NewFactory(bot.factoryAddr, bot.client)
	if err != nil {
		return common.HexToAddress("0x"), err
	}

	pairAddr, err := factory.GetPair(&bind.CallOpts{}, bot.inTokenAddr, bot.outTokenAddr)
	if err != nil {
		return common.HexToAddress("0x"), err
	}

	return pairAddr, nil
}

func (bot *Bot) getDecimal(tokenAddr common.Address) (uint8, error) {
	token, err := erc20.NewErc20(tokenAddr, bot.client)
	if err != nil {
		return 0, err
	}

	decimal, err := token.Decimals(&bind.CallOpts{})
	if err != nil {
		return 0, err
	}

	return decimal, nil
}

func (bot *Bot) getBalanceOf(tokenAddr common.Address, walletAddr common.Address) (*big.Int, error) {
	token, err := erc20.NewErc20(tokenAddr, bot.client)
	if err != nil {
		return nil, err
	}

	balance, err := token.BalanceOf(&bind.CallOpts{}, walletAddr)
	if err != nil {
		return nil, err
	}

	return balance, nil
}

// func (bot *Bot) approve(amountIn *big.Int, transOpt *bind.TransactOpts) error {
// 	token, err := erc20.NewErc20(bot.inTokenAddr, bot.client)
// 	if err != nil {
// 		return err
// 	}

// 	// allowance, err:= token.Allowance(&bind.CallOpts{},bot.userAddr, bot.routerAddr)
// 	// if err!= nil {
// 	// 	return err
// 	// }

// 	// if allowance.Cmp(amountIn)<0{

// 	// }

// 	// max := big.Ne(wInt1.1579209e77)

// 	// tx := token.Approve(transOpt, bot.routerAddr, max)

// }

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

func (bot *Bot) swap(to common.Address, amount *big.Int, deadline *big.Int, isETH bool, transOpt *bind.TransactOpts) (*types.Transaction, error) {
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
			return nil, err
		}
	} else {
		tx, err = router.SwapExactTokensForTokens(transOpt, amount, big.NewInt(0), path, to, deadline)
		if err != nil {
			return nil, err
		}
	}
	return tx, nil
}

func (bot *Bot) getNonce() (uint64, error) {
	nonce, err := bot.client.PendingNonceAt(context.Background(), bot.userAddr)
	if err != nil {
		return 0, err
	}

	return nonce, nil
}

func (bot *Bot) getTxReceipt(tx *types.Transaction) (*types.Receipt, error) {
	receipt, err := bot.client.TransactionReceipt(context.Background(), tx.Hash())
	if err != nil {
		return nil, err
	}
	return receipt, nil
}

func process(rpc string, privateKeyString string, routerV2AddrString string, factoryAddrString string, isETH bool, inTokenAddrString string, outTokenAddrString string, amountInString string, gasPriceString string) error {

	// fmt.Print("RPC endpoint: ")
	// rpc := inputHandler()

	client, err := ethclient.Dial(rpc)
	if err != nil {
		return err
	}

	fmt.Println("we have a connection")

	// fmt.Print("Enter Your Private Key: ")
	// privateKeyString := inputHandler()

	if strings.Contains(privateKeyString, "0x") {
		privateKeyString = privateKeyString[2:]
	}

	privateKey, err := crypto.HexToECDSA(privateKeyString)
	if err != nil {
		fmt.Println("invalid private key")
		return err
	}

	// fmt.Print("Router V2 Address: ")
	// routerV2AddrString := inputHandler()
	routerAddr := common.HexToAddress(routerV2AddrString)

	// fmt.Print("Factory Address: ")
	// factoryAddrString := inputHandler()
	factoryAddr := common.HexToAddress(factoryAddrString)

	// var isETH = false
	// fmt.Print("is ETH?(Y/N): ")
	// isETHString := inputHandler()
	// if isETHString == "y" {
	// 	isETH = true
	// }

	// var inTokenAddr common.Address
	// fmt.Print("inToken Address: ")
	// inTokenAddrString := inputHandler()
	inTokenAddr := common.HexToAddress(inTokenAddrString)

	// fmt.Print("outToken Address: ")
	// outTokenAddrString := inputHandler()
	outTokenAddr := common.HexToAddress(outTokenAddrString)

	// fmt.Print("amount In: ")
	// amountInString := inputHandler()
	amount, err := strconv.ParseFloat(amountInString, 64)
	if err != nil {
		log.Fatal("input amount error")
	}

	// fmt.Print("gas price: ")
	// gasPriceString := inputHandler()
	gasPrice, err := strconv.ParseFloat(gasPriceString, 64)
	if err != nil {
		log.Fatal("input amount error")
	}

	bot := NewBot(client, privateKey, inTokenAddr, outTokenAddr, routerAddr, factoryAddr)

	for {
		pairAddr, err := bot.getPairAddr()
		if err != nil {
			return err
		}
		if pairAddr == ZERO_ADDRESS {
			fmt.Println(time.Now(), " pool not created yet!")
			continue
		} else {
			fmt.Println("pool address is: ", pairAddr.Hex())
			for {
				poolBalance, err := bot.getBalanceOf(bot.inTokenAddr, pairAddr)
				if err != nil {
					return err
				}

				if poolBalance.Cmp(big.NewInt(0)) > 0 {
					nonce, err := bot.getNonce()
					if err != nil {
						return err
					}

					var bigIntAmount *big.Int
					if isETH {
						amount *= 1000000000000000000
						bigIntAmount = big.NewInt(int64(amount))

					} else {
						decimal, err := bot.getDecimal(bot.inTokenAddr)
						if err != nil {
							return err
						}
						amount *= float64(decimal)
						bigIntAmount = big.NewInt(int64(amount))
					}

					transOpt := bot.getTransOpt(gasPrice, 3000000, nonce)
					tx, err := bot.swap(bot.userAddr, bigIntAmount, big.NewInt(99999999999), isETH, transOpt)
					if err != nil {
						return err
					}

					receipt, err := bot.getTxReceipt(tx)
					if err != nil {
						return err
					}
					fmt.Println(receipt.Status)
					amountOut, err := bot.getBalanceOf(bot.outTokenAddr, bot.userAddr)
					if err != nil {
						return err
					}
					bitFloatAmountOut := new(big.Float).SetInt(amountOut)
					decimal, err := bot.getDecimal(bot.outTokenAddr)
					if err != nil {
						return err
					}
					outTokenDecimal := big.NewFloat(float64(decimal))
					bigAmountOut := new(big.Float).Quo(bitFloatAmountOut, outTokenDecimal)

					fmt.Println("you get: ", bigAmountOut)
					return nil
				} else {
					fmt.Println(time.Now(), "pool is still empty")
				}
			}
		}

	}

}

func main() {
	a := app.New()
	win := a.NewWindow("ICO BOT")

	rpcEndPoint := widget.NewEntry()
	privateKey := widget.NewEntry()
	tokenInAddrString := widget.NewEntry()
	tokenOutAddrString := widget.NewEntry()
	amountInString := widget.NewEntry()
	gasPriceString := widget.NewEntry()

	win.SetContent(widget.NewVBox(
		widget.NewLabel("rpc end point"),
		rpcEndPoint,
		widget.NewLabel("private Key"),
		privateKey,
		widget.NewLabel("token in address"),
		tokenInAddrString,
		widget.NewLabel("token out address"),
		tokenOutAddrString,
		widget.NewLabel("amount in"),
		amountInString,
		widget.NewLabel("gas price"),
		gasPriceString,
		widget.NewButton("Start", func() {
			isETH := false
			if strings.Compare(strings.ToLower(tokenInAddrString.Text), strings.ToLower("0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c")) == 0 {
				isETH = true
			}
			go process(rpcEndPoint.Text, privateKey.Text, "0x10ED43C718714eb63d5aA57B78B54704E256024E", "0xcA143Ce32Fe78f1f7019d7d551a6402fC5350c73", isETH, tokenInAddrString.Text, tokenOutAddrString.Text, amountInString.Text, gasPriceString.Text)
			// if err != nil {
			// 	fmt.Println(err.Error())
			// }
		}),
		widget.NewButton("Quit", func() {
			a.Quit()
		}),
	))
	win.Resize(fyne.NewSize(640, 800))
	win.ShowAndRun()
}
