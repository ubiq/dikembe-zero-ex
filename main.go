package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	ERC721OrdersFeature "github.com/ubiq/dikembe-discord/contracts"
	ubiq "github.com/ubiq/go-ubiq/v7"
	"github.com/ubiq/go-ubiq/v7/accounts/abi"
	"github.com/ubiq/go-ubiq/v7/common"
	"github.com/ubiq/go-ubiq/v7/core/types"
	"github.com/ubiq/go-ubiq/v7/ethclient"
	"github.com/ubiq/go-ubiq/v7/params"
)

type payload struct {
	Username  string  `json:"username"`
	AvatarURL string  `json:"avatar_url"`
	Embeds    []embed `json:"embeds"`
}

type embed struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

var (
	wsRPCURL   string
	webhookURL string

	avatarUsername string
	avatarURL      string

	lastTXID string

	// 0x address
	address string = "0x19aaD856cE8c4C7e813233b21d56dA97796cC052"

	tokenMap = map[string]string{
		"0x1e1628A35C82169F876F97C9CE5B6533895c2B22": "CHIMP",
		"0x0e2fbBA88C5E526f5160Af1b9Ad79a20130b2216": "GB89",
		"0x81f1a6e026d49c2260a8D6D8e14Bca1628c1Df43": "nCeption",
	}
)

func init() {
	flag.StringVar(&wsRPCURL, "wsRPCURL", "ws://127.0.0.1:8589", "WS RPC URL")
	flag.StringVar(&webhookURL, "webhookURL", "https://discord.com/api/webhooks/", "Webhook URL")
	flag.StringVar(&avatarUsername, "avatarUsername", "Jawa", "Avatar username")
	flag.StringVar(&avatarURL, "avatarURL", "https://i.pinimg.com/originals/73/48/64/734864cdf9a657fd65b9ae79120739d3.jpg", "Avatar image URL")
	flag.Parse()
}

func main() {
	client, err := ethclient.Dial(wsRPCURL)
	if err != nil {
		log.Fatalln(err)
	}
	defer client.Close()

	subch := make(chan types.Log)

	go func() {
		for i := 0; ; i++ {
			if i > 0 {
				time.Sleep(2 * time.Second)
			}
			subscribeFilterLogs(client, subch)
		}
	}()

	erc721OrderCancelled := common.HexToHash("0xa015ad2dc32f266993958a0fd9884c746b971b254206f3478bc43e2f125c7b9e")
	erc721OrderFilled := common.HexToHash("0x50273fa02273cceea9cf085b42de5c8af60624140168bd71357db833535877af")
	erc721OrderPreSigned := common.HexToHash("0x8c5d0c41fb16a7317a6c55ff7ba93d9d74f79e434fefa694e50d6028afbfa3f0")

	contractAbi, err := abi.JSON(strings.NewReader(string(ERC721OrdersFeature.ERC721OrdersFeatureABI)))
	if err != nil {
		log.Fatal(err)
	}

	for vLog := range subch {
		if len(vLog.Topics) == 0 {
			continue
		}
		if vLog.TxHash.String() == lastTXID {
			log.Println("Duplicate TX: ", lastTXID)
			continue
		}
		switch vLog.Topics[0].Hex() {
		case erc721OrderCancelled.Hex():
			var event ERC721OrdersFeature.ERC721OrdersFeatureERC721OrderCancelled
			err := contractAbi.UnpackIntoInterface(&event, "ERC721OrderCancelled", vLog.Data)
			if err != nil {
				log.Fatal(err)
			}

			// TODO - Format this event
			fmt.Printf("erc721OrderCancelled - %+v\n", event)
			//msg := ""
			//postEvent(msg, vLog.BlockNumber, vLog.TxHash.String())
			lastTXID = vLog.TxHash.String()
		case erc721OrderFilled.Hex():
			var event ERC721OrdersFeature.ERC721OrdersFeatureERC721OrderFilled
			err := contractAbi.UnpackIntoInterface(&event, "ERC721OrderFilled", vLog.Data)
			if err != nil {
				log.Fatal(err)
			}

			msg := ""
			msg = fmt.Sprintf("SALE! - %s ID: %d ; %.8f UBQ ; Seller %s Buyer %s",
				tokenMap[event.Erc721Token.String()], event.Erc721TokenId,
				weiToEther(event.Erc20TokenAmount), event.Maker, event.Taker)
			postEvent(msg, vLog.BlockNumber, vLog.TxHash.String())
			lastTXID = vLog.TxHash.String()
		case erc721OrderPreSigned.Hex():
			var event ERC721OrdersFeature.ERC721OrdersFeatureERC721OrderPreSigned
			err := contractAbi.UnpackIntoInterface(&event, "ERC721OrderPreSigned", vLog.Data)
			if err != nil {
				log.Fatal(err)
			}

			msg := ""
			msg = fmt.Sprintf("LIST! - %s ID: %d ; %.8f UBQ ; Seller %s",
				tokenMap[event.Erc721Token.String()], event.Erc721TokenId,
				weiToEther(event.Erc20TokenAmount), event.Maker)
			postEvent(msg, vLog.BlockNumber, vLog.TxHash.String())
			lastTXID = vLog.TxHash.String()
		}
	}
}

func subscribeFilterLogs(client *ethclient.Client, subch chan types.Log) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	contractAddress := common.HexToAddress(address)
	query := ubiq.FilterQuery{
		Addresses: []common.Address{
			contractAddress,
		},
	}

	// Subscribe to log events
	sub, err := client.SubscribeFilterLogs(ctx, query, subch)
	if err != nil {
		log.Println("subscribe error:", err)
		return
	}

	// The connection is established now.
	// Update the channel
	var logs types.Log
	subch <- logs

	// The subscription will deliver events to the channel. Wait for the
	// subscription to end for any reason, then loop around to re-establish
	// the connection.
	log.Println("connection lost: ", <-sub.Err())
}

func postEvent(msg string, block uint64, txid string) {
	title := fmt.Sprintf("Block: %d TX: %.30s...", block, txid)
	titleURL := fmt.Sprintf("https://ubiqscan.io/tx/%s", txid)

	blockEmbed := embed{Title: title, URL: titleURL, Description: msg}
	embeds := []embed{blockEmbed}
	jsonReq := payload{Username: avatarUsername, AvatarURL: avatarURL, Embeds: embeds}

	jsonStr, _ := json.Marshal(jsonReq)
	log.Println("JSON POST:", string(jsonStr))

	req, _ := http.NewRequest("POST", webhookURL, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()
}

func weiToEther(wei *big.Int) *big.Float {
	if len(wei.Bits()) == 0 {
		return nil
	}
	return new(big.Float).Quo(new(big.Float).SetInt(wei), big.NewFloat(params.Ether))
}
