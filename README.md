# Dikembe ZeroEx - Ubiq Discord Bot

This Go-based Discord bot monitors Ubiq blockchain events related to specific ERC721 token trade events and sends notifications to a Discord channel via webhooks. The bot listens for events such as ERC721 order fills and pre-signs and posts transaction details to Discord.

This is based on the Ubiq of the [0x v4 NFT contracts](https://github.com/ubiq/ubiq-contracts-zero-ex-nft).

The code could be cleaned up to make it more generic but it also serves as good sample code for working with Solidity-Go bindings (generated with Abigen) and calling Solidity contracts from Go.

## Features

- Monitors ERC721 token and 0x trade events on the Ubiq blockchain.
- Sends detailed notifications to a Discord channel via webhooks.
- Supports specific ERC721 tokens with custom handling.

## Prerequisites

- A Ubiq node (for RPC and WebSocket connections)
- Discord account and webhook URL
