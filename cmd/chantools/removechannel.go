package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/guggero/chantools/lnd"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/spf13/cobra"
)

type removeChannelCommand struct {
	ChannelDB string
	Channel   string

	cmd *cobra.Command
}

func newRemoveChannelCommand() *cobra.Command {
	cc := &removeChannelCommand{}
	cc.cmd = &cobra.Command{
		Use:   "removechannel",
		Short: "Remove a single channel from the given channel DB",
		RunE:  cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.ChannelDB, "channeldb", "", "lnd channel.backup file to "+
			"remove the channel from",
	)
	cc.cmd.Flags().StringVar(
		&cc.Channel, "channel", "", "channel to remove from the DB "+
			"file, identified by its channel point "+
			"(<txid>:<txindex>)",
	)

	return cc.cmd
}

func (c *removeChannelCommand) Execute(_ *cobra.Command, _ []string) error {
	// Check that we have a channel DB.
	if c.ChannelDB == "" {
		return fmt.Errorf("channel DB is required")
	}
	db, err := lnd.OpenDB(c.ChannelDB, false)
	if err != nil {
		return fmt.Errorf("error opening channel DB: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Errorf("Error closing DB: %v", err)
		}
	}()

	parts := strings.Split(c.Channel, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid channel point format: %v", c.Channel)
	}
	hash, err := chainhash.NewHashFromStr(parts[0])
	if err != nil {
		return err
	}
	index, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return err
	}

	return removeChannel(db, &wire.OutPoint{
		Hash:  *hash,
		Index: uint32(index),
	})
}

func removeChannel(db *channeldb.DB, chanPoint *wire.OutPoint) error {
	dbChan, err := db.FetchChannel(*chanPoint)
	if err != nil {
		return err
	}

	if err := dbChan.MarkBorked(); err != nil {
		return err
	}

	// Abandoning a channel is a three step process: remove from the open
	// channel state, remove from the graph, remove from the contract
	// court. Between any step it's possible that the users restarts the
	// process all over again. As a result, each of the steps below are
	// intended to be idempotent.
	return db.AbandonChannel(chanPoint, uint32(100000))
}