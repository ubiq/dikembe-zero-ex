package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/ubiq/bishop-discord/nft"
	ERC721OrdersFeature "github.com/ubiq/dikembe-discord/contracts"
	ubiq "github.com/ubiq/go-ubiq/v7"
	"github.com/ubiq/go-ubiq/v7/accounts/abi"
	"github.com/ubiq/go-ubiq/v7/common"
	"github.com/ubiq/go-ubiq/v7/core/types"
	"github.com/ubiq/go-ubiq/v7/ethclient"
	"github.com/ubiq/go-ubiq/v7/params"
)

var (
	rpcURL       string
	wsRPCURL     string
	webhookID    string
	webhookToken string

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
	flag.StringVar(&rpcURL, "rpcURL", "http://127.0.0.1:8588", "RPC URL")
	flag.StringVar(&wsRPCURL, "wsRPCURL", "ws://127.0.0.1:8589", "WS RPC URL")
	flag.StringVar(&webhookID, "webhookID", "", "Webhook ID")
	flag.StringVar(&webhookToken, "webhookToken", "", "Webhook Token")
	flag.StringVar(&avatarUsername, "avatarUsername", "Jawa", "Avatar username")
	flag.StringVar(&avatarURL, "avatarURL", "https://i.pinimg.com/originals/3e/6f/39/3e6f39b3d264bbbf6479553383adc905.jpg", "Avatar image URL")
	flag.Parse()
}

func main() {
	clientWS, err := ethclient.Dial(wsRPCURL)
	if err != nil {
		log.Fatalln(err)
	}
	defer clientWS.Close()

	clientRPC, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatal(err)
	}
	defer clientRPC.Close()

	subch := make(chan types.Log)

	go func() {
		for i := 0; ; i++ {
			if i > 0 {
				time.Sleep(2 * time.Second)
			}
			subscribeFilterLogs(clientWS, subch)
		}
	}()

	//erc721OrderCancelled := common.HexToHash("0xa015ad2dc32f266993958a0fd9884c746b971b254206f3478bc43e2f125c7b9e")
	erc721OrderFilled := common.HexToHash("0x50273fa02273cceea9cf085b42de5c8af60624140168bd71357db833535877af")
	erc721OrderPreSigned := common.HexToHash("0x8c5d0c41fb16a7317a6c55ff7ba93d9d74f79e434fefa694e50d6028afbfa3f0")

	contractAbi, err := abi.JSON(strings.NewReader(string(ERC721OrdersFeature.ERC721OrdersFeatureMetaData.ABI)))
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
		case erc721OrderFilled.Hex():
			var event ERC721OrdersFeature.ERC721OrdersFeatureERC721OrderFilled
			err := contractAbi.UnpackIntoInterface(&event, "ERC721OrderFilled", vLog.Data)
			if err != nil {
				log.Fatal(err)
			}

			// Query the blockchain for the amount sent as event log does not show the Fees
			tx, _, err := clientRPC.TransactionByHash(context.Background(), vLog.TxHash)
			if err != nil {
				log.Fatal(err)
			}

			msg := fmt.Sprintf("**Seller:** %s\n**Buyer:** %s\n**Price:** %.5f UBQ",
				shortAddress(event.Maker), shortAddress(event.Taker), weiToEther(tx.Value()))
			title := fmt.Sprintf("🤑 SALE! - %s #%d",
				tokenMap[event.Erc721Token.String()], event.Erc721TokenId)
			webhookExecuteTradeNFTEvent(msg, vLog.TxHash.String(), title,
				tokenMap[event.Erc721Token.String()], event.Erc721TokenId)
			lastTXID = vLog.TxHash.String()
		case erc721OrderPreSigned.Hex():
			var event ERC721OrdersFeature.ERC721OrdersFeatureERC721OrderPreSigned
			err := contractAbi.UnpackIntoInterface(&event, "ERC721OrderPreSigned", vLog.Data)
			if err != nil {
				log.Fatal(err)
			}

			// Process Fee
			fees := big.NewInt(0)
			erc20TokenAmount := big.NewInt(0)
			if len(event.Fees) > 0 {
				for _, fee := range event.Fees {
					fees.Add(fees, fee.Amount)
				}
			}
			erc20TokenAmount.Add(fees, event.Erc20TokenAmount)

			msg := fmt.Sprintf("**Seller:** %s\n**Price:** %.5f UBQ",
				shortAddress(event.Maker), weiToEther(erc20TokenAmount))
			title := fmt.Sprintf("🫰 LIST! - %s #%d",
				tokenMap[event.Erc721Token.String()], event.Erc721TokenId)
			webhookExecuteTradeNFTEvent(msg, vLog.TxHash.String(), title,
				tokenMap[event.Erc721Token.String()], event.Erc721TokenId)
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

func webhookExecuteTradeNFTEvent(msg string, txid string, title string, token string, tokenID *big.Int) {
	titleURL := fmt.Sprintf("https://ubiqscan.io/tx/%s", txid)
	var iNFT nft.NFT
	switch token {
	case "CHIMP":
		iNFT = nft.HandleChimp(rpcURL, tokenID)
	case "GB89":
		iNFT = nft.HandleGB89(rpcURL, tokenID)
	case "nCeption":
		iNFT = nft.HandleNception(rpcURL, tokenID)
	}

	session, _ := discordgo.New("")
	msgembed := discordgo.MessageEmbed{
		URL:         titleURL,
		Type:        discordgo.EmbedTypeLink,
		Title:       title,
		Description: msg,
		Color:       16750848,
		Image: &discordgo.MessageEmbedImage{
			URL: "attachment://output.png",
		},
	}
	attachment := discordgo.File{
		Name:        "output.png",
		ContentType: "image/png",
		Reader:      bytes.NewReader(iNFT.Picture),
	}
	webhook := &discordgo.WebhookParams{
		Username:  avatarUsername,
		AvatarURL: avatarURL,
		Embeds:    []*discordgo.MessageEmbed{&msgembed},
		Files:     []*discordgo.File{&attachment},
	}

	_, err := session.WebhookExecute(webhookID, webhookToken, true, webhook)
	if err != nil {
		log.Println(err)
	}
}

func weiToEther(wei *big.Int) *big.Float {
	if len(wei.Bits()) == 0 {
		return nil
	}
	return new(big.Float).Quo(new(big.Float).SetInt(wei), big.NewFloat(params.Ether))
}

func shortAddress(a common.Address) string {
	aS := a.String()
	return fmt.Sprintf("%s...%s", aS[:4], aS[len(aS)-4:])
}
