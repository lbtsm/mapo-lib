# mapo-lib

Common Go libraries for MAPO ecosystem projects.

## Packages

### alarm

Slack webhook alarm with message deduplication.

#### Features

- Slack incoming webhook via [go-slack](https://github.com/slack-go/slack) SDK
- Duplicate message suppression within configurable interval (default 5 min)
- Global default instance (`Init` / `Send`) and custom instances (`New`)
- Fallback to environment variables `alarm_webhooks` and `alarm_prefix`
- Auto-appends local IP when prefix is not set
- Graceful degradation: logs locally when webhook URL is empty

#### Install

```bash
go get github.com/mapprotocol/mapo-lib/alarm
```

#### Usage

**Global instance (simple)**

```go
import "github.com/mapprotocol/mapo-lib/alarm"

// Initialize once at startup
alarm.Init("production", "https://hooks.slack.com/services/xxx")

// Send from anywhere, no need to pass the instance around
alarm.Send(ctx, "balance too low on bsc chain")
```

**Custom instance**

```go
a := alarm.New("staging", "https://hooks.slack.com/services/yyy",
    alarm.WithInterval(10 * time.Minute),
)
a.Send(ctx, "node out of sync")
```

**Zero config (environment variables)**

```bash
export alarm_webhooks="https://hooks.slack.com/services/xxx"
export alarm_prefix="my-service"
```

```go
alarm.Init("", "")
alarm.Send(ctx, "something happened")
// Slack message: "my-service something happened"
```

If both webhook URL and env are empty, messages are logged to stdout instead of sent to Slack.

#### Environment Variables

| Variable | Description |
|---|---|
| `alarm_webhooks` | Fallback Slack webhook URL |
| `alarm_prefix` | Fallback message prefix |

### keystore

Ethereum keystore decryption with terminal password prompt.

#### Install

```bash
go get github.com/mapprotocol/mapo-lib/keystore
```

#### Usage

```go
import "github.com/mapprotocol/mapo-lib/keystore"

// Decrypt an Ethereum keystore file (prompts for password on first use, then caches it)
key, err := keystore.KeypairFromEth("/path/to/keyfile.json")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Address:", key.Address.Hex())
```

Password can also be set via the `KEYSTORE_PASSWORD` environment variable.

### abi

Wrapper around go-ethereum's ABI with convenient helpers.

#### Install

```bash
go get github.com/mapprotocol/mapo-lib/abi
```

#### Usage

```go
import mapoabi "github.com/mapprotocol/mapo-lib/abi"

a, _ := mapoabi.New(erc20AbiJSON)

// Pack a method call
data, _ := a.PackInput("transfer", toAddr, amount)

// Unpack method output
var supply *big.Int
a.UnpackOutput("totalSupply", &supply, outputBytes)

// Decode raw transaction input
methodName, values, _ := a.UnpackInputData(tx.Data())

// Unpack event log
var event TransferEvent
a.UnpackLog(&event, "Transfer", log)

// Lookup
id, _ := a.GetMethodID("transfer")     // 4-byte selector
topic, _ := a.GetEventID("Transfer")   // event topic hash
a.HasMethod("approve")                 // true/false
```

### contract

Read-only contract caller built on go-ethereum's ethclient.

#### Install

```bash
go get github.com/mapprotocol/mapo-lib/contract
```

#### Usage

```go
import (
    mapoabi "github.com/mapprotocol/mapo-lib/abi"
    "github.com/mapprotocol/mapo-lib/contract"
    "github.com/ethereum/go-ethereum/ethclient"
)

client, _ := ethclient.Dial("https://rpc.example.com")
a, _ := mapoabi.New(abiJSON)
c := contract.New(client, []common.Address{contractAddr}, a)

// Call a view function
var supply *big.Int
c.Call("totalSupply", &supply, 0)

// Call at a specific block
c.CallAt(ctx, "balanceOf", &balance, 0, blockNum, addr)
```

## License

MIT
