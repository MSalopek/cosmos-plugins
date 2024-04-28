# cosmos-plugins
cosmos-sdk plugins for versions >= 0.50.x

## storechanges (blockheader, KV store changes)

Implements `ABCIListener`.

Allows saving `StoreKVPair` and the cometBFT header to a file for further processing.
```go
type ABCIListener interface {
  ListenFinalizeBlock(ctx context.Context, req abci.RequestFinalizeBlock, res abci.ResponseFinalizeBlock) error
  ListenCommit(ctx context.Context, res abci.ResponseCommit, changeSet []*StoreKVPair) error
}
```

This tool requires access to a cometBFT node via RPC. It cannot function without one.

### Usage

You must compile the plugin before use and configure your cosmos-sdk node (`simd` or other) to use your plugin.

```shell
## build
cd cosmos-plugins
go build -o ./cmd/storechanges/storechanges ./cmd/storechanges/main.go

# move it if you want
# mv storechanges <path>

## export path to plugin so the chain app can find it
export COSMOS_SDK_ABCI="<abs_path_to_parent_dir>/cmd/storechanges/storechanges"
```

Configuring your node:
In your `app.toml` add these sections:
```toml
[streaming]

[streaming.abci]

# List of kv store keys to stream out via gRPC.
# The store key names MUST match the module's StoreKey name.
#
# Example:
# ["acc", "bank", "gov", "staking", "mint"[,...]]
# ["*"] to expose all keys.
keys = ["bank", "transfer"]

plugin = "abci"

# stop-node-on-err specifies whether to stop the node on message delivery error.
stop-node-on-err = true
```


### Plugin configuration

Plugin is configured using environment variables. Here are the available options:
```shell
# check .envexample
COMET_RPC_URL = "tcp://localhost:26657" # comet node used for fetching block data 
STREAMING_DIR = "~/.storechanges" # output files will be stored to this directory
```

The listed values are also used as defaults.

### Start node and plugins
Create a an env file before starting your node and export the variables.
```shell
cp .env.example .env
# make changes
```

Export the variables before starting your node (in the same terminal session) and start your node.
```shell
set -a
. .env
set +a

simd start
```

### App configuration

If your node does not register streaming listeners you will need to add this snippet to your `app.go`:
```diff
// app/app.go
func NewSimApp(
  logger log.Logger,
  db dbm.DB,
  traceStore io.Writer,
  loadLatest bool,
  appOpts servertypes.AppOptions,
  baseAppOptions ...func(*baseapp.BaseApp),
) *SimApp {
  // ...

  keys := storetypes.NewKVStoreKeys(
    authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey, crisistypes.StoreKey,
    minttypes.StoreKey, distrtypes.StoreKey, slashingtypes.StoreKey,
    govtypes.StoreKey, paramstypes.StoreKey, consensusparamtypes.StoreKey, upgradetypes.StoreKey, feegrant.StoreKey,
    evidencetypes.StoreKey, circuittypes.StoreKey,
    authzkeeper.StoreKey, nftkeeper.StoreKey, group.StoreKey,
)

+  // register streaming services
+  if err := bApp.RegisterStreamingServices(appOpts, keys); err != nil {
+    panic(err)
+  }
    
   // ...
}
```
