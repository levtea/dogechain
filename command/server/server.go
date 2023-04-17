package server

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/dogechain-lab/dogechain/command"
	"github.com/dogechain-lab/dogechain/command/helper"
	"github.com/dogechain-lab/dogechain/crypto"
	"github.com/dogechain-lab/dogechain/helper/daemon"
	"github.com/dogechain-lab/dogechain/helper/gasprice"
	"github.com/dogechain-lab/dogechain/helper/kvdb"
	"github.com/dogechain-lab/dogechain/network"
	"github.com/dogechain-lab/dogechain/server"
	"github.com/dogechain-lab/dogechain/txpool"
	"github.com/howeyc/gopass"
	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	serverCmd := &cobra.Command{
		Use:     "server",
		Short:   "The default command that starts the Dogechain-Lab Dogechain client, by bootstrapping all modules together",
		PreRunE: runPreRun,
		Run:     runCommand,
	}

	helper.RegisterGRPCAddressFlag(serverCmd)
	helper.RegisterLegacyGRPCAddressFlag(serverCmd)
	helper.RegisterJSONRPCFlag(serverCmd)
	helper.RegisterGraphQLFlag(serverCmd)

	setFlags(serverCmd)

	return serverCmd
}

func setFlags(cmd *cobra.Command) {
	defaultConfig := DefaultConfig()

	// basic flags
	{
		cmd.Flags().StringVar(
			&params.configPath,
			configFlag,
			"",
			"the path to the CLI config. Supports .json and .hcl",
		)

		cmd.Flags().StringVar(
			&params.rawConfig.DataDir,
			dataDirFlag,
			defaultConfig.DataDir,
			"the data directory used for storing Dogechain-Lab Dogechain client data",
		)

		cmd.Flags().StringVar(
			&params.rawConfig.GenesisPath,
			genesisPathFlag,
			defaultConfig.GenesisPath,
			"the genesis file used for starting the chain",
		)

		cmd.Flags().StringVar(
			&params.rawConfig.RestoreFile,
			restoreFlag,
			"",
			"the path to the archive blockchain data to restore on initialization",
		)
	}

	// block flags
	{
		cmd.Flags().Uint64Var(
			&params.rawConfig.BlockTime,
			blockTimeFlag,
			defaultConfig.BlockTime,
			"minimum block time in seconds (at least 1s)",
		)
		cmd.Flags().BoolVar(
			&params.rawConfig.BlockBroadcast,
			blockBroadcastFlag,
			false,
			"(deprecated) enable block broadcast when syncing",
		)
	}

	// endpoint flags
	{
		cmd.Flags().Uint64Var(
			&params.rawConfig.JSONRPCBatchRequestLimit,
			jsonRPCBatchRequestLimitFlag,
			defaultConfig.JSONRPCBatchRequestLimit,
			"the max length to be considered when handling json-rpc batch requests",
		)

		cmd.Flags().Uint64Var(
			&params.rawConfig.JSONRPCBlockRangeLimit,
			jsonRPCBlockRangeLimitFlag,
			defaultConfig.JSONRPCBlockRangeLimit,
			"the max block range to be considered when executing json-rpc requests "+
				"that consider fromBlock/toBlock values (e.g. eth_getLogs)",
		)

		cmd.Flags().BoolVar(
			&params.rawConfig.EnableWS,
			enableWSFlag,
			false,
			"the flag indicating that node enable websocket service",
		)

		cmd.Flags().BoolVar(
			&params.rawConfig.EnableGraphQL,
			enableGraphQLFlag,
			false,
			"the flag indicating that node enable graphql service",
		)

		cmd.Flags().StringVar(
			&params.rawConfig.JSONNamespace,
			jsonrpcNamespaceFlag,
			defaultConfig.JSONNamespace,
			"the jsonrpc endpoint namespaces should be enabled "+
				"(eth, net, web3, txpool, debug. concatenate with commas or * for all)",
		)
	}

	// leveldb flags
	{
		cmd.Flags().IntVar(
			&params.leveldbCacheSize,
			leveldbCacheFlag,
			kvdb.DefaultLevelDBCache,
			"the size of the leveldb cache in MB",
		)

		cmd.Flags().IntVar(
			&params.leveldbHandles,
			leveldbHandlesFlag,
			kvdb.DefaultLevelDBHandles,
			"the number of handles to leveldb open files",
		)

		cmd.Flags().IntVar(
			&params.leveldbBloomKeyBits,
			leveldbBloomKeyBitsFlag,
			kvdb.DefaultLevelDBBloomKeyBits,
			"the bits of leveldb bloom filters",
		)

		cmd.Flags().IntVar(
			&params.leveldbTableSize,
			leveldbTableSizeFlag,
			kvdb.DefaultLevelDBCompactionTableSize,
			"the leveldb 'sorted table' size in MB",
		)

		cmd.Flags().IntVar(
			&params.leveldbTotalTableSize,
			leveldbTotalTableSizeFlag,
			kvdb.DefaultLevelDBCompactionTotalSize,
			"limits leveldb total size of 'sorted table' for each level in MB",
		)

		cmd.Flags().BoolVar(
			&params.leveldbNoSync,
			leveldbNoSyncFlag,
			kvdb.DefaultLevelDBNoSync,
			"leveldb nosync allows completely disable fsync",
		)

		cmd.Flags().StringArrayVar(
			&params.kvAddress,
			kvConfigFlag,
			[]string{},
			"ankr kv address",
		)

		cmd.Flags().StringVar(
			&params.kvPassword,
			kvConfigPasswordFlag,
			"",
			"ankr kv password",
		)

		cmd.Flags().StringArrayVar(
			&params.pubAddress,
			pubConfigFlag,
			[]string{},
			"ankr pub pub address",
		)

		cmd.Flags().StringVar(
			&params.pubPassword,
			pubConfigPasswordFlag,
			"",
			"ankr pub password",
		)
	}

	// log flags
	{
		cmd.Flags().StringVar(
			&params.rawConfig.LogLevel,
			command.LogLevelFlag,
			defaultConfig.LogLevel,
			"the log level for console output",
		)

		cmd.Flags().StringVar(
			&params.rawConfig.LogFilePath,
			logFileLocationFlag,
			defaultConfig.LogFilePath,
			"write all logs to the file at specified location instead of writing them to console",
		)
	}

	// miner flags
	{
		cmd.Flags().BoolVar(
			&params.rawConfig.ShouldSeal,
			sealFlag,
			false,
			"the flag indicating that the client should seal blocks",
		)

		cmd.Flags().StringVar(
			&params.rawConfig.BlockGasTarget,
			blockGasTargetFlag,
			strconv.FormatUint(0, 10),
			"the target block gas limit for the chain. If omitted, the value of the parent block is used",
		)

		cmd.Flags().BoolVar(
			&params.isDaemon,
			daemonFlag,
			false,
			"the flag indicating that the server ran as daemon",
		)

		cmd.Flags().StringVar(
			&params.rawConfig.SecretsConfigPath,
			secretsConfigFlag,
			"",
			"the path to the SecretsManager config file. Used for Hashicorp Vault. "+
				"If omitted, the local FS secrets manager is used",
		)
	}

	// network flags
	{
		cmd.Flags().StringVar(
			&params.rawConfig.Network.IgnoreDiscoverCIDR,
			command.IgnoreDiscoverCIDRFlag,
			defaultConfig.Network.IgnoreDiscoverCIDR,
			"The comma separated list of CIDR ranges to ignore when discovering peers",
		)

		cmd.Flags().BoolVar(
			&params.rawConfig.Network.NoDiscover,
			command.NoDiscoverFlag,
			defaultConfig.Network.NoDiscover,
			"prevent the client from discovering other peers (default: false)",
		)

		cmd.Flags().Int64Var(
			&params.rawConfig.Network.MaxPeers,
			maxPeersFlag,
			-1,
			"the client's max number of peers allowed",
		)
		// override default usage value
		cmd.Flag(maxPeersFlag).DefValue = fmt.Sprintf("%d", defaultConfig.Network.MaxPeers)

		cmd.Flags().Int64Var(
			&params.rawConfig.Network.MaxInboundPeers,
			maxInboundPeersFlag,
			-1,
			"the client's max number of inbound peers allowed",
		)
		// override default usage value
		cmd.Flag(maxInboundPeersFlag).DefValue = fmt.Sprintf("%d", defaultConfig.Network.MaxInboundPeers)

		cmd.Flags().Int64Var(
			&params.rawConfig.Network.MaxOutboundPeers,
			maxOutboundPeersFlag,
			-1,
			"the client's max number of outbound peers allowed",
		)
		// override default usage value
		cmd.Flag(maxOutboundPeersFlag).DefValue = fmt.Sprintf("%d", defaultConfig.Network.MaxOutboundPeers)

		cmd.Flags().StringVar(
			&params.rawConfig.Network.Libp2pAddr,
			libp2pAddressFlag,
			fmt.Sprintf("127.0.0.1:%d", network.DefaultLibp2pPort),
			"the address and port for the libp2p service",
		)

		cmd.Flags().StringVar(
			&params.rawConfig.Network.NatAddr,
			natFlag,
			"",
			"the external address (address:port), as can be seen by peers",
		)

		cmd.Flags().StringVar(
			&params.rawConfig.Network.DNSAddr,
			dnsFlag,
			"",
			"the host DNS address which can be used by a remote peer for connection",
		)

		cmd.Flags().StringArrayVar(
			&params.corsAllowedOrigins,
			corsOriginFlag,
			defaultConfig.Headers.AccessControlAllowOrigins,
			"the CORS header indicating whether any JSON-RPC response can be shared with the specified origin",
		)
	}

	// telemetry flags
	{
		cmd.Flags().StringVar(
			&params.rawConfig.Telemetry.PrometheusAddr,
			prometheusAddressFlag,
			"",
			"the address and port for the prometheus instrumentation service (address:port). "+
				"If only port is defined (:port) it will bind to 0.0.0.0:port",
		)

		cmd.Flags().BoolVar(
			&params.rawConfig.Telemetry.EnableIOTimer,
			enableIOTimerFlag,
			false,
			"enable IO timer metrics",
		)
	}

	// txpool flags
	{
		cmd.Flags().Uint64Var(
			&params.rawConfig.TxPool.PriceLimit,
			priceLimitFlag,
			0,
			fmt.Sprintf(
				"the minimum gas price limit to enforce for acceptance into the pool (default %d)",
				defaultConfig.TxPool.PriceLimit,
			),
		)

		cmd.Flags().Uint64Var(
			&params.rawConfig.TxPool.MaxSlots,
			maxSlotsFlag,
			txpool.DefaultMaxSlots,
			"maximum slots in the pool",
		)

		// pruning outdated account flags
		{
			cmd.Flags().Uint64Var(
				&params.rawConfig.TxPool.PruneTickSeconds,
				pruneTickSecondsFlag,
				txpool.DefaultPruneTickSeconds,
				"tick seconds for pruning account future transactions in the pool",
			)

			cmd.Flags().Uint64Var(
				&params.rawConfig.TxPool.PromoteOutdateSeconds,
				promoteOutdateSecondsFlag,
				txpool.DefaultPromoteOutdateSeconds,
				"account in the pool not promoted for a long time would be pruned",
			)
		}
	}

	{ // gas price oracle flags
		cmd.Flags().IntVar(
			&params.rawConfig.GPO.Blocks,
			gpoBlocksFlag,
			gasprice.Defaults.Blocks,
			"number of recent blocks to blocks check for gas prices",
		)

		cmd.Flags().IntVar(
			&params.rawConfig.GPO.Percentile,
			gpoPercentileFlag,
			gasprice.Defaults.Percentile,
			"the given percentile of a set of recent transaction gas prices",
		)

		cmd.Flags().Int64Var(
			&params.gpoMaxGasPrice,
			gpoMaxGasPriceFlag,
			gasprice.Defaults.MaxPrice.Int64(),
			"maximum gasprice recommended by gas price oracle",
		)

		cmd.Flags().Int64Var(
			&params.gpoIgnoreGasPrice,
			gpoIgnoreGasPriceFlag,
			gasprice.Defaults.IgnorePrice.Int64(),
			"gas price below which gpo will ignore transactions",
		)
	}

	setDevFlags(cmd)
}

func setDevFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(
		&params.isDevMode,
		devFlag,
		false,
		"should the client start in dev mode (default false)",
	)

	_ = cmd.Flags().MarkHidden(devFlag)

	cmd.Flags().Uint64Var(
		&params.devInterval,
		devIntervalFlag,
		0,
		"the client's dev notification interval in seconds (default 1)",
	)

	helper.RegisterPprofFlag(cmd)
	helper.RegisterJaegerFlag(cmd)

	_ = cmd.Flags().MarkHidden(devIntervalFlag)
}

func runPreRun(cmd *cobra.Command, _ []string) error {
	// Set the grpc, json and graphql ip:port bindings
	// The config file will have precedence over --flag
	params.setRawGRPCAddress(helper.GetGRPCAddress(cmd))
	params.setRawJSONRPCAddress(helper.GetJSONRPCAddress(cmd))
	params.setRawGraphQLAddress(helper.GetGraphQLAddress(cmd))

	// Check if the config file has been specified
	// Config file settings will override JSON-RPC and GRPC address values
	if isConfigFileSpecified(cmd) {
		if err := params.initConfigFromFile(); err != nil {
			return err
		}
	}

	if err := params.validateFlags(); err != nil {
		return err
	}

	if err := params.initRawParams(); err != nil {
		return err
	}

	return nil
}

func isConfigFileSpecified(cmd *cobra.Command) bool {
	return cmd.Flags().Changed(configFlag)
}

func askForConfirmation() string {
	reader := bufio.NewReader(os.Stdin)

	for {
		privateKeyRaw, err := gopass.GetPasswdPrompt("Enter ValidatorKey:", true, os.Stdin, os.Stdout)
		if err != nil {
			log.Println("Parent process ", os.Getpid(), " passwd prompt err:", err)
		}

		privateKey, err := crypto.BytesToPrivateKey(privateKeyRaw)
		if err != nil {
			log.Println("Parent process ", os.Getpid(), " input to private key, err:", err)
		}

		validatorKeyAddr := crypto.PubKeyToAddress(&privateKey.PublicKey)

		log.Println("Parent process ", os.Getpid(), " passwd prompt, ValidatorKey len:", len(params.validatorKey),
			", ValidatorKeyAddr: ", validatorKeyAddr.String())

		fmt.Printf("ValidatorKey Address: %s [y/n]: ", validatorKeyAddr.String())

		response, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}

		response = strings.ToLower(strings.TrimSpace(response))

		if response == "y" || response == "yes" {
			return string(privateKeyRaw)
		} else if response == "n" || response == "no" {
			continue
		}
	}
}

func runCommand(cmd *cobra.Command, _ []string) {
	command.InitializePprofServer(cmd)
	outputter := command.InitializeOutputter(cmd)

	log.Println("Main process run isDaemon:", params.isDaemon)

	// Launch daemons
	if params.isDaemon {
		// First time, daemonIdx is empty
		daemonIdx := os.Getenv(daemon.EnvDaemonIdx)
		if len(daemonIdx) == 0 {
			params.validatorKey = askForConfirmation()
		} else {
			data, err := ioutil.ReadAll(os.Stdin)
			if err != nil {
				log.Println("Child process ", os.Getpid(), " read pipe data err: ", err)
			} else {
				log.Println("Child process ", os.Getpid(), " read pipe data: ", len(string(data)))
				params.validatorKey = string(data)
			}
		}

		// Create a daemon object
		newDaemon := daemon.NewDaemon(daemon.DaemonLog)
		newDaemon.MaxCount = daemon.MaxCount
		newDaemon.ValidatorKey = params.validatorKey

		// Execute daemon mode
		newDaemon.Run()

		// When params.isDaemon = true,
		// the following code will only be executed by the final child process,
		// and neither the main process nor the daemon will execute
		log.Println("Child process ", os.Getpid(), "start...")
		log.Println("Child process ", os.Getpid(), "isDaemon: ", params.isDaemon)
		log.Println("Child process ", os.Getpid(), "ValidatorKey len: ", len(params.validatorKey))
	}

	// pprof flag
	params.rawConfig.EnablePprof = helper.GetPprofFlag(cmd)

	// jaeger flag
	if enableJaeger, jaegerURL := helper.GetJaegerFlag(cmd); enableJaeger {
		params.rawConfig.Telemetry.EnableJaeger = enableJaeger
		params.rawConfig.Telemetry.JaegerURL = jaegerURL
	}

	if err := runServerLoop(params.generateConfig(), outputter); err != nil {
		outputter.SetError(err)
		outputter.WriteOutput()

		return
	}
}

func runServerLoop(
	config *server.Config,
	outputter command.OutputFormatter,
) error {
	serverInstance, err := server.NewServer(config)
	if err != nil {
		return err
	}

	return helper.HandleSignals(serverInstance.Close, outputter)
}
