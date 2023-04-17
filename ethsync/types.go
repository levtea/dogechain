package ethsync

import (
	"math/big"
	"strconv"
	"strings"

	"github.com/dogechain-lab/dogechain/helper/hex"
	"github.com/dogechain-lab/dogechain/types"
)

// For union type of transaction and types.Hash
type transactionOrHash interface {
	getHash() types.Hash
}

type transaction struct {
	Nonce       argUint64      `json:"nonce"`
	GasPrice    argBig         `json:"gasPrice"`
	Gas         argUint64      `json:"gas"`
	To          *types.Address `json:"to"`
	Value       argBig         `json:"value"`
	Input       argBytes       `json:"input"`
	V           argBig         `json:"v"`
	R           argBig         `json:"r"`
	S           argBig         `json:"s"`
	Hash        types.Hash     `json:"hash"`
	From        types.Address  `json:"from"`
	BlockHash   *types.Hash    `json:"blockHash"`
	BlockNumber *argUint64     `json:"blockNumber"`
	TxIndex     *argUint64     `json:"transactionIndex"`
}

func (t transaction) getHash() types.Hash { return t.Hash }

// Redefine to implement getHash() of transactionOrHash
type transactionHash types.Hash

func (h transactionHash) getHash() types.Hash { return types.Hash(h) }

func (h transactionHash) MarshalText() ([]byte, error) {
	return []byte(types.Hash(h).String()), nil
}

func toPendingTransaction(t *types.Transaction) *transaction {
	return toTransaction(t, nil, nil, nil)
}

func toTransaction(
	t *types.Transaction,
	blockNumber *argUint64,
	blockHash *types.Hash,
	txIndex *int,
) *transaction {
	res := &transaction{
		Nonce:    argUint64(t.Nonce),
		GasPrice: argBig(*t.GasPrice),
		Gas:      argUint64(t.Gas),
		To:       t.To,
		Value:    argBig(*t.Value),
		Input:    t.Input,
		V:        argBig(*t.V),
		R:        argBig(*t.R),
		S:        argBig(*t.S),
		Hash:     t.Hash(),
		From:     t.From,
	}

	if blockNumber != nil {
		res.BlockNumber = blockNumber
	}

	if blockHash != nil {
		res.BlockHash = blockHash
	}

	if txIndex != nil {
		res.TxIndex = argUintPtr(uint64(*txIndex))
	}

	return res
}

type block struct {
	ParentHash      types.Hash          `json:"parentHash"`
	Sha3Uncles      types.Hash          `json:"sha3Uncles"`
	Miner           types.Address       `json:"miner"`
	StateRoot       types.Hash          `json:"stateRoot"`
	TxRoot          types.Hash          `json:"transactionsRoot"`
	ReceiptsRoot    types.Hash          `json:"receiptsRoot"`
	LogsBloom       types.Bloom         `json:"logsBloom"`
	Difficulty      argUint64           `json:"difficulty"`
	TotalDifficulty argUint64           `json:"totalDifficulty"`
	Size            argUint64           `json:"size"`
	Number          argUint64           `json:"number"`
	GasLimit        argUint64           `json:"gasLimit"`
	GasUsed         argUint64           `json:"gasUsed"`
	Timestamp       argUint64           `json:"timestamp"`
	ExtraData       argBytes            `json:"extraData"`
	MixHash         types.Hash          `json:"mixHash"`
	Nonce           types.Nonce         `json:"nonce"`
	Hash            types.Hash          `json:"hash"`
	Transactions    []transactionOrHash `json:"transactions"`
	Uncles          []types.Hash        `json:"uncles"`
}

func toBlock(b *types.Block, fullTx bool) *block {
	h := b.Header
	res := &block{
		ParentHash:      h.ParentHash,
		Sha3Uncles:      h.Sha3Uncles,
		Miner:           h.Miner,
		StateRoot:       h.StateRoot,
		TxRoot:          h.TxRoot,
		ReceiptsRoot:    h.ReceiptsRoot,
		LogsBloom:       h.LogsBloom,
		Difficulty:      argUint64(h.Difficulty),
		TotalDifficulty: argUint64(h.Difficulty), // not needed for POS
		Size:            argUint64(b.Size()),
		Number:          argUint64(h.Number),
		GasLimit:        argUint64(h.GasLimit),
		GasUsed:         argUint64(h.GasUsed),
		Timestamp:       argUint64(h.Timestamp),
		ExtraData:       argBytes(h.ExtraData),
		MixHash:         h.MixHash,
		Nonce:           h.Nonce,
		Hash:            h.Hash,
		Transactions:    []transactionOrHash{},
		Uncles:          []types.Hash{},
	}

	for idx, txn := range b.Transactions {
		if fullTx {
			res.Transactions = append(
				res.Transactions,
				toTransaction(
					txn,
					argUintPtr(b.Number()),
					argHashPtr(b.Hash()),
					&idx,
				),
			)
		} else {
			res.Transactions = append(
				res.Transactions,
				transactionHash(txn.Hash()),
			)
		}
	}

	for _, uncle := range b.Uncles {
		res.Uncles = append(res.Uncles, uncle.Hash)
	}

	return res
}

type receipt struct {
	Root              types.Hash     `json:"root"`
	CumulativeGasUsed argUint64      `json:"cumulativeGasUsed"`
	LogsBloom         types.Bloom    `json:"logsBloom"`
	Logs              []*Log         `json:"logs"`
	Status            argUint64      `json:"status"`
	TxHash            types.Hash     `json:"transactionHash"`
	TxIndex           argUint64      `json:"transactionIndex"`
	BlockHash         types.Hash     `json:"blockHash"`
	BlockNumber       argUint64      `json:"blockNumber"`
	GasUsed           argUint64      `json:"gasUsed"`
	ContractAddress   *types.Address `json:"contractAddress"`
	FromAddr          types.Address  `json:"from"`
	ToAddr            *types.Address `json:"to"`
}

func toReceipt(r *types.Receipt, txn *types.Transaction, block *types.Block, indx int64) *receipt {

	logs := make([]*Log, len(r.Logs))
	for logIndx, elem := range r.Logs {
		e := elem
		logs[logIndx] = &Log{
			Address:     e.Address,
			Topics:      e.Topics,
			Data:        []byte("0x" + hex.EncodeToString(e.Data)),
			BlockHash:   block.Hash(),
			BlockNumber: argUint64(block.Number()),
			TxHash:      txn.Hash(),
			TxIndex:     argUint64(indx),
			LogIndex:    argUint64(logIndx),
			Removed:     false,
		}
	}

	return &receipt{
		Root:              r.Root,
		CumulativeGasUsed: argUint64(r.CumulativeGasUsed),
		LogsBloom:         r.LogsBloom,
		Status:            argUint64(*r.Status),
		TxHash:            txn.Hash(),
		TxIndex:           argUint64(indx),
		BlockHash:         block.Hash(),
		BlockNumber:       argUint64(block.Number()),
		GasUsed:           argUint64(r.GasUsed),
		ContractAddress:   r.ContractAddress,
		FromAddr:          txn.From,
		ToAddr:            txn.To,
		Logs:              logs,
	}
}

func toReceiptJSONRPC(receipts []*types.Receipt, block *types.Block, txIndex int64) *receipt {

	txn := block.Transactions[txIndex]
	raw := receipts[txIndex]

	logs := make([]*Log, len(raw.Logs))
	for indx, elem := range raw.Logs {
		logs[indx] = &Log{
			Address:     elem.Address,
			Topics:      elem.Topics,
			Data:        argBytes(elem.Data),
			BlockHash:   block.Hash(),
			BlockNumber: argUint64(block.Number()),
			TxHash:      txn.Hash(),
			TxIndex:     argUint64(txIndex),
			LogIndex:    argUint64(indx),
			Removed:     false,
		}
	}

	res := &receipt{
		Root:              raw.Root,
		CumulativeGasUsed: argUint64(raw.CumulativeGasUsed),
		LogsBloom:         raw.LogsBloom,
		Status:            argUint64(*raw.Status),
		TxHash:            txn.Hash(),
		TxIndex:           argUint64(txIndex),
		BlockHash:         block.Hash(),
		BlockNumber:       argUint64(block.Number()),
		GasUsed:           argUint64(raw.GasUsed),
		ContractAddress:   raw.ContractAddress,
		FromAddr:          txn.From,
		ToAddr:            txn.To,
		Logs:              logs,
	}

	return res
}

type Log struct {
	Address     types.Address `json:"address"`
	Topics      []types.Hash  `json:"topics"`
	Data        argBytes      `json:"data"`
	BlockNumber argUint64     `json:"blockNumber"`
	TxHash      types.Hash    `json:"transactionHash"`
	TxIndex     argUint64     `json:"transactionIndex"`
	BlockHash   types.Hash    `json:"blockHash"`
	LogIndex    argUint64     `json:"logIndex"`
	Removed     bool          `json:"removed"`
}

type argBig big.Int

func argBigPtr(b *big.Int) *argBig {
	v := argBig(*b)

	return &v
}

func (a *argBig) UnmarshalText(input []byte) error {
	buf, err := decodeToHex(input)
	if err != nil {
		return err
	}

	b := new(big.Int)
	b.SetBytes(buf)
	*a = argBig(*b)

	return nil
}

func (a argBig) MarshalText() ([]byte, error) {
	b := (*big.Int)(&a)

	return []byte("0x" + b.Text(16)), nil
}

func argAddrPtr(a types.Address) *types.Address {
	return &a
}

func argHashPtr(h types.Hash) *types.Hash {
	return &h
}

type argUint64 uint64

func argUintPtr(n uint64) *argUint64 {
	v := argUint64(n)

	return &v
}

func (u argUint64) MarshalText() ([]byte, error) {
	buf := make([]byte, 2, 10)
	copy(buf, `0x`)
	buf = strconv.AppendUint(buf, uint64(u), 16)

	return buf, nil
}

func (u *argUint64) UnmarshalText(input []byte) error {
	str := strings.TrimPrefix(string(input), "0x")
	num, err := strconv.ParseUint(str, 16, 64)

	if err != nil {
		return err
	}

	*u = argUint64(num)

	return nil
}

type argBytes []byte

func argBytesPtr(b []byte) *argBytes {
	bb := argBytes(b)

	return &bb
}

func (b argBytes) MarshalText() ([]byte, error) {
	return encodeToHex(b), nil
}

func (b *argBytes) UnmarshalText(input []byte) error {
	hh, err := decodeToHex(input)
	if err != nil {
		return nil
	}

	aux := make([]byte, len(hh))
	copy(aux[:], hh[:])
	*b = aux

	return nil
}

func decodeToHex(b []byte) ([]byte, error) {
	str := string(b)
	str = strings.TrimPrefix(str, "0x")

	if len(str)%2 != 0 {
		str = "0" + str
	}

	return hex.DecodeString(str)
}

func encodeToHex(b []byte) []byte {
	str := hex.EncodeToString(b)
	if len(str)%2 != 0 {
		str = "0" + str
	}

	return []byte("0x" + str)
}

// txnArgs is the transaction argument for the rpc endpoints
type txnArgs struct {
	From     *types.Address
	To       *types.Address
	Gas      *argUint64
	GasPrice *argBytes
	Value    *argBytes
	Data     *argBytes
	Input    *argBytes
	Nonce    *argUint64
}

type progression struct {
	Type          string `json:"type"`
	StartingBlock string `json:"startingBlock"`
	CurrentBlock  string `json:"currentBlock"`
	HighestBlock  string `json:"highestBlock"`
}
