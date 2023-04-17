package server

import (
	"errors"
	"log"
	"net"
	"strings"

	"github.com/hashicorp/go-hclog"

	"github.com/dogechain-lab/dogechain/chain"
	"github.com/dogechain-lab/dogechain/network"
	"github.com/dogechain-lab/dogechain/secrets"
	"github.com/dogechain-lab/dogechain/server"
	"github.com/multiformats/go-multiaddr"
)

const (
	configFlag                   = "config"
	genesisPathFlag              = "chain"
	dataDirFlag                  = "data-dir"
	leveldbCacheFlag             = "leveldb.cache-size"
	leveldbHandlesFlag           = "leveldb.handles"
	leveldbBloomKeyBitsFlag      = "leveldb.bloom-bits"
	leveldbTableSizeFlag         = "leveldb.table-size"
	leveldbTotalTableSizeFlag    = "leveldb.total-table-size"
	leveldbNoSyncFlag            = "leveldb.nosync"
	libp2pAddressFlag            = "libp2p"
	prometheusAddressFlag        = "prometheus"
	enableIOTimerFlag            = "prometheus-io-timer"
	natFlag                      = "nat"
	dnsFlag                      = "dns"
	sealFlag                     = "seal"
	maxPeersFlag                 = "max-peers"
	maxInboundPeersFlag          = "max-inbound-peers"
	maxOutboundPeersFlag         = "max-outbound-peers"
	priceLimitFlag               = "price-limit"
	maxSlotsFlag                 = "max-slots"
	pruneTickSecondsFlag         = "prune-tick-seconds"
	promoteOutdateSecondsFlag    = "promote-outdate-seconds"
	blockGasTargetFlag           = "block-gas-target"
	secretsConfigFlag            = "secrets-config"
	restoreFlag                  = "restore"
	blockTimeFlag                = "block-time"
	devIntervalFlag              = "dev-interval"
	devFlag                      = "dev"
	corsOriginFlag               = "access-control-allow-origins"
	daemonFlag                   = "daemon"
	logFileLocationFlag          = "log-to"
	enableGraphQLFlag            = "enable-graphql"
	jsonRPCBatchRequestLimitFlag = "json-rpc-batch-request-limit"
	jsonRPCBlockRangeLimitFlag   = "json-rpc-block-range-limit"
	jsonrpcNamespaceFlag         = "json-rpc-namespace"
	enableWSFlag                 = "enable-ws"
	blockBroadcastFlag           = "block-broadcast"
	gpoBlocksFlag                = "gpo.blocks"
	gpoPercentileFlag            = "gpo.percentile"
	gpoMaxGasPriceFlag           = "gpo.maxprice"
	gpoIgnoreGasPriceFlag        = "gpo.ignoreprice"
	// ankr config
	kvConfigFlag          = "kv.address"
	pubConfigFlag         = "pub.address"
	kvConfigPasswordFlag  = "kv.password"
	pubConfigPasswordFlag = "pub.password"
)

const (
	unsetPeersValue = -1
)

var (
	params = &serverParams{
		rawConfig: &Config{
			Telemetry: &Telemetry{},
			Network:   &Network{},
			TxPool:    &TxPool{},
		},
	}
)

var (
	errInvalidPeerParams = errors.New("both max-peers and max-inbound/outbound flags are set")
	errInvalidNATAddress = errors.New("could not parse NAT address (ip:port)")
)

type serverParams struct {
	rawConfig  *Config
	configPath string

	leveldbCacheSize      int
	leveldbHandles        int
	leveldbBloomKeyBits   int
	leveldbTableSize      int
	leveldbTotalTableSize int
	leveldbNoSync         bool

	libp2pAddress *net.TCPAddr

	prometheusAddress   *net.TCPAddr
	prometheusIOMetrics bool

	natAddress     *net.TCPAddr
	dnsAddress     multiaddr.Multiaddr
	grpcAddress    *net.TCPAddr
	jsonRPCAddress *net.TCPAddr
	graphqlAddress *net.TCPAddr

	blockGasTarget uint64
	devInterval    uint64
	isDevMode      bool
	isDaemon       bool
	validatorKey   string

	corsAllowedOrigins []string

	genesisConfig *chain.Chain
	secretsConfig *secrets.SecretsManagerConfig

	logFileLocation string

	// gas price oracle
	gpoMaxGasPrice    int64
	gpoIgnoreGasPrice int64

	// ankr
	kvAddress   []string
	kvPassword  string
	pubAddress  []string
	pubPassword string
}

func (p *serverParams) validateFlags() error {
	// Validate the max peers configuration
	if p.isMaxPeersSet() && p.isPeerRangeSet() {
		return errInvalidPeerParams
	}

	return nil
}

func (p *serverParams) isLogFileLocationSet() bool {
	return p.rawConfig.LogFilePath != ""
}

func (p *serverParams) isMaxPeersSet() bool {
	return p.rawConfig.Network.MaxPeers != unsetPeersValue
}

func (p *serverParams) isPeerRangeSet() bool {
	return p.rawConfig.Network.MaxInboundPeers != unsetPeersValue ||
		p.rawConfig.Network.MaxOutboundPeers != unsetPeersValue
}

func (p *serverParams) isSecretsConfigPathSet() bool {
	return p.rawConfig.SecretsConfigPath != ""
}

func (p *serverParams) isPrometheusAddressSet() bool {
	return p.rawConfig.Telemetry.PrometheusAddr != ""
}

func (p *serverParams) isNATAddressSet() bool {
	return p.rawConfig.Network.NatAddr != ""
}

func (p *serverParams) isDNSAddressSet() bool {
	return p.rawConfig.Network.DNSAddr != ""
}

func (p *serverParams) isDevConsensus() bool {
	return server.ConsensusType(p.genesisConfig.Params.GetEngine()) == server.DevConsensus
}

func (p *serverParams) getRestoreFilePath() *string {
	if p.rawConfig.RestoreFile != "" {
		return &p.rawConfig.RestoreFile
	}

	return nil
}

func (p *serverParams) setRawGRPCAddress(grpcAddress string) {
	p.rawConfig.GRPCAddr = grpcAddress
}

func (p *serverParams) setRawJSONRPCAddress(jsonRPCAddress string) {
	p.rawConfig.JSONRPCAddr = jsonRPCAddress
}

func (p *serverParams) setRawGraphQLAddress(graphqlAddress string) {
	p.rawConfig.GraphQLAddr = graphqlAddress
}

func (p *serverParams) generateConfig() *server.Config {
	chainCfg := p.genesisConfig

	// Replace block gas limit
	if p.blockGasTarget > 0 {
		chainCfg.Params.BlockGasTarget = p.blockGasTarget
	}

	// namespace
	ns := strings.Split(p.rawConfig.JSONNamespace, ",")

	// ignore cidr
	cidrList := strings.Split(p.rawConfig.Network.IgnoreDiscoverCIDR, ",")
	ingoreCIDRs := []*net.IPNet{}

	for _, cidrStr := range cidrList {
		cidrStr = strings.TrimSpace(cidrStr)
		if cidrStr == "" {
			continue
		}

		_, ipnet, err := net.ParseCIDR(cidrStr)
		if err != nil {
			log.Printf("CIDR formart error: %s \n", err)

			continue
		}

		ingoreCIDRs = append(ingoreCIDRs, ipnet)
	}

	return &server.Config{
		Chain: chainCfg,
		JSONRPC: &server.JSONRPC{
			JSONRPCAddr:              p.jsonRPCAddress,
			AccessControlAllowOrigin: p.corsAllowedOrigins,
			BatchLengthLimit:         p.rawConfig.JSONRPCBatchRequestLimit,
			BlockRangeLimit:          p.rawConfig.JSONRPCBlockRangeLimit,
			JSONNamespace:            ns,
			EnableWS:                 p.rawConfig.EnableWS,
			EnablePprof:              p.rawConfig.EnablePprof,
		},
		EnableGraphQL: p.rawConfig.EnableGraphQL,
		GraphQL: &server.GraphQL{
			GraphQLAddr:              p.graphqlAddress,
			AccessControlAllowOrigin: p.corsAllowedOrigins,
			BlockRangeLimit:          p.rawConfig.JSONRPCBlockRangeLimit,
			EnablePprof:              p.rawConfig.EnablePprof,
		},
		GRPCAddr:   p.grpcAddress,
		LibP2PAddr: p.libp2pAddress,
		Telemetry: &server.Telemetry{
			PrometheusAddr:  p.prometheusAddress,
			EnableIOMetrics: p.prometheusIOMetrics,
			EnableJaeger:    p.rawConfig.Telemetry.EnableJaeger,
			JaegerURL:       p.rawConfig.Telemetry.JaegerURL,
		},
		Network: &network.Config{
			NoDiscover:         p.rawConfig.Network.NoDiscover,
			DiscoverIngoreCIDR: ingoreCIDRs,

			Addr:             p.libp2pAddress,
			NatAddr:          p.natAddress,
			DNS:              p.dnsAddress,
			DataDir:          p.rawConfig.DataDir,
			MaxPeers:         p.rawConfig.Network.MaxPeers,
			MaxInboundPeers:  p.rawConfig.Network.MaxInboundPeers,
			MaxOutboundPeers: p.rawConfig.Network.MaxOutboundPeers,
			Chain:            p.genesisConfig,
		},
		DataDir:               p.rawConfig.DataDir,
		Seal:                  p.rawConfig.ShouldSeal,
		PriceLimit:            p.rawConfig.TxPool.PriceLimit,
		MaxSlots:              p.rawConfig.TxPool.MaxSlots,
		PruneTickSeconds:      p.rawConfig.TxPool.PruneTickSeconds,
		PromoteOutdateSeconds: p.rawConfig.TxPool.PromoteOutdateSeconds,
		SecretsManager:        p.secretsConfig,
		RestoreFile:           p.getRestoreFilePath(),
		LeveldbOptions: &server.LeveldbOptions{
			CacheSize:           p.leveldbCacheSize,
			Handles:             p.leveldbHandles,
			BloomKeyBits:        p.leveldbBloomKeyBits,
			CompactionTableSize: p.leveldbTableSize,
			CompactionTotalSize: p.leveldbTotalTableSize,
			NoSync:              p.leveldbNoSync,
		},
		BlockTime:      p.rawConfig.BlockTime,
		LogLevel:       hclog.LevelFromString(p.rawConfig.LogLevel),
		LogFilePath:    p.logFileLocation,
		Daemon:         p.isDaemon,
		ValidatorKey:   p.validatorKey,
		BlockBroadcast: p.rawConfig.BlockBroadcast,
		GasPriceOracle: p.rawConfig.GPO,
		// ankr
		KvAddress:   p.kvAddress,
		KvPassword:  p.kvPassword,
		PubAddress:  p.pubAddress,
		PubPassword: p.pubPassword,
	}
}
