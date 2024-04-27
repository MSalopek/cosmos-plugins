package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/hashicorp/go-plugin"

	"cosmossdk.io/errors"
	streamingabci "cosmossdk.io/store/streaming/abci"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"

	"github.com/cosmos/cosmos-sdk/client"
	svc "github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"

	store "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
)

type FilePlugin struct {
	BlockHeight  int64
	Cdc          codec.Codec
	CometClient  *client.Context
	StreamingDir string
}

func (a *FilePlugin) ListenFinalizeBlock(ctx context.Context, req abci.RequestFinalizeBlock, res abci.ResponseFinalizeBlock) error {
	a.BlockHeight = req.Height
	return nil
}

func (a *FilePlugin) ListenCommit(ctx context.Context, res abci.ResponseCommit, changeSet []*store.StoreKVPair) error {
	_, block, err := svc.GetProtoBlock(ctx, *a.CometClient, &a.BlockHeight)
	if err != nil {
		return err
	}

	bz, err := a.Cdc.Marshal(&block.Header)
	if err != nil {
		return err
	}

	if err := a.writeLengthPrefixedFile(fmt.Sprintf("block-%d-header", block.Header.Height), bz); err != nil {
		return err
	}

	var blockDataBuf bytes.Buffer
	if err := a.writeBlockData(&blockDataBuf, changeSet); err != nil {
		return err
	}

	if err := a.writeLengthPrefixedFile(fmt.Sprintf("block-%d-data", block.Header.Height), blockDataBuf.Bytes()); err != nil {
		return err
	}

	return nil
}

func (a *FilePlugin) writeBlockData(writer io.Writer, kps []*store.StoreKVPair) error {
	for i := range kps {
		bz, err := a.Cdc.MarshalLengthPrefixed(kps[i])
		if err != nil {
			return err
		}

		if _, err := writer.Write(bz); err != nil {
			return err
		}
	}

	return nil
}

func (a *FilePlugin) writeLengthPrefixedFile(fname string, data []byte) (err error) {
	fpath := fmt.Sprintf("%s/%s", a.StreamingDir, fname)

	var f *os.File
	f, err = os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return errors.Wrapf(err, "open file failed: %s", fpath)
	}

	defer func() {
		// avoid overriding the real error with file close error
		if err1 := f.Close(); err1 != nil && err == nil {
			err = errors.Wrapf(err, "close file failed: %s", fpath)
		}
	}()
	_, err = f.Write(store.Uint64ToBigEndian(uint64(len(data))))
	if err != nil {
		return errors.Wrapf(err, "write length prefix failed: %s", fpath)
	}

	_, err = f.Write(data)
	if err != nil {
		return errors.Wrapf(err, "write block data failed: %s", fpath)
	}

	return err
}

func main() {
	cometRPC := os.Getenv("COMET_RPC_URL")
	if cometRPC == "" {
		cometRPC = "tcp://localhost:26657"
	}

	streamingDir := os.Getenv("STREAMING_DIR")
	if streamingDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			panic(fmt.Sprintf("error getting home directory: %s", err))
		}
		streamingDir = path.Join(home, ".storechanges")
	}
	if err := os.MkdirAll(streamingDir, 0o755); err != nil {
		panic(fmt.Sprintf("error creating streaming directory: %s", err))
	}

	reg := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(reg)

	// connect comet client for getting block data
	clientCtx := client.Context{}
	rpcclient, err := client.NewClientFromNode(cometRPC)
	if err != nil {
		panic(fmt.Sprintf("error getting comet client: %s", err))
	}
	clientCtx = clientCtx.WithClient(rpcclient)

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: streamingabci.Handshake,
		Plugins: map[string]plugin.Plugin{
			"abci": &streamingabci.ListenerGRPCPlugin{
				Impl: &FilePlugin{
					Cdc:          cdc,
					CometClient:  &clientCtx,
					StreamingDir: streamingDir,
				},
			},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
