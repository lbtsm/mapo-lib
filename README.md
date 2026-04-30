# mapo-lib

Common Go libraries for MAPO ecosystem projects.

## Packages

### alarm

Slack webhook alarm with message deduplication.

#### Features

- Slack incoming webhook via [go-slack](https://github.com/slack-go/slack) SDK
- Duplicate message suppression within configurable interval (default 5 min)
- Global default instance (`Init` / `Send`) and custom instances (`New`)
- `Sender` interface for easy mocking in consumer tests
- Fallback to environment variables `alarm_webhooks` and `alarm_prefix`
- Auto-appends local IP when prefix is not set
- Graceful degradation: logs locally when webhook URL is empty
- Thread-safe under concurrent send/init

#### Install

```bash
go get github.com/lbtsm/mapo-lib/alarm
```

#### Usage

**Global instance (simple)**

```go
import "github.com/lbtsm/mapo-lib/alarm"

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

Ethereum keystore decryption with optional terminal password prompt.

#### Install

```bash
go get github.com/lbtsm/mapo-lib/keystore
```

#### Usage

```go
import "github.com/lbtsm/mapo-lib/keystore"

// Non-interactive: pass the password directly. Suitable for servers and tests.
key, err := keystore.KeypairFromEthWithPassword("/path/to/keyfile.json", []byte("pwd"))

// Interactive: cache → KEYSTORE_PASSWORD env → terminal prompt.
key, err := keystore.KeypairFromEth("/path/to/keyfile.json")

fmt.Println("Address:", key.Address.Hex())
```

The password cache is shared across calls and protected by a mutex. Failed decryption does not poison the cache.

### abi

Wrapper around go-ethereum's ABI with convenient helpers and sentinel errors.

#### Install

```bash
go get github.com/lbtsm/mapo-lib/abi
```

#### Usage

```go
import mapoabi "github.com/lbtsm/mapo-lib/abi"

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

// Sentinel errors for programmatic handling
if errors.Is(err, mapoabi.ErrMethodNotFound) { ... }
```

### contract

Read-only contract caller built on go-ethereum's ethclient.

#### Install

```bash
go get github.com/lbtsm/mapo-lib/contract
```

#### Usage

```go
import (
    mapoabi "github.com/lbtsm/mapo-lib/abi"
    "github.com/lbtsm/mapo-lib/contract"
    "github.com/ethereum/go-ethereum/ethclient"
)

client, _ := ethclient.Dial("https://rpc.example.com")
a, _ := mapoabi.New(abiJSON)
c := contract.New(client, []common.Address{contractAddr}, a)

// Preferred: address-based call
var supply *big.Int
c.CallContract(ctx, contractAddr, "totalSupply", &supply)

// Index-based call (legacy, bounds-checked — returns ErrIndexOutOfRange on bad idx)
c.Call("totalSupply", &supply, 0)

// Call at a specific block
c.CallAt(ctx, "balanceOf", &balance, 0, blockNum, addr)

// Set the From address used in CallMsg
c2 := contract.New(client, addrs, a, contract.WithFrom(myAddr))
```

## License

MIT
